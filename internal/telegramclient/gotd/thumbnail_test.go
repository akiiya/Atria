package gotd

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	"strings"
	"testing"

	"github.com/gotd/td/tg"
)

// makeStripped 构造一个合法的去头 JPEG 载荷。
// entropy 是熵编码数据，实际内容由调用方决定。
func makeStripped(w, h byte, entropy []byte) []byte {
	out := []byte{0x01, w, h}
	return append(out, entropy...)
}

func TestDecodeStrippedThumb_RejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too short", []byte{0x01, 0x20}},
		{"wrong version marker", []byte{0x02, 0x20, 0x20, 0xaa}},
		{"zero version marker", []byte{0x00, 0x20, 0x20, 0xaa}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeStrippedThumb(tt.in); got != nil {
				t.Errorf("期望 nil，实际得到 %d 字节", len(got))
			}
		})
	}
}

func TestDecodeStrippedThumb_ProducesJPEGMarkers(t *testing.T) {
	out := decodeStrippedThumb(makeStripped(0x20, 0x18, []byte{0xaa, 0xbb, 0xcc}))
	if out == nil {
		t.Fatal("合法输入不应返回 nil")
	}

	// JPEG 必须以 SOI (FFD8) 开头、EOI (FFD9) 结尾
	if !bytes.HasPrefix(out, []byte{0xff, 0xd8}) {
		t.Errorf("缺少 JPEG SOI 标记，实际开头=% x", out[:2])
	}
	if !bytes.HasSuffix(out, []byte{0xff, 0xd9}) {
		t.Errorf("缺少 JPEG EOI 标记，实际结尾=% x", out[len(out)-2:])
	}
}

func TestDecodeStrippedThumb_WritesDimensions(t *testing.T) {
	const w, h = 0x28, 0x1e
	out := decodeStrippedThumb(makeStripped(w, h, []byte{0xaa}))
	if out == nil {
		t.Fatal("合法输入不应返回 nil")
	}

	if out[strippedWidthOffset] != w {
		t.Errorf("宽度未写入正确位置：期望 %#x，实际 %#x", w, out[strippedWidthOffset])
	}
	if out[strippedHeightOffset] != h {
		t.Errorf("高度未写入正确位置：期望 %#x，实际 %#x", h, out[strippedHeightOffset])
	}
}

func TestDecodeStrippedThumb_PreservesEntropyData(t *testing.T) {
	entropy := []byte{0xde, 0xad, 0xbe, 0xef}
	out := decodeStrippedThumb(makeStripped(0x10, 0x10, entropy))
	if out == nil {
		t.Fatal("合法输入不应返回 nil")
	}

	// 熵数据应原样出现在 header 之后、footer 之前
	body := out[len(strippedJPEGHeader) : len(out)-len(strippedJPEGFooter)]
	if !bytes.Equal(body, entropy) {
		t.Errorf("熵数据被改动：期望 % x，实际 % x", entropy, body)
	}
}

// 真实的 Telegram 去头缩略图样本，验证还原结果能被标准 JPEG 解码器解析。
const realStrippedSample = "AQoKlAA="

func TestDecodeStrippedThumb_DecodableAsImage(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(realStrippedSample)
	if err != nil {
		t.Fatalf("测试样本解码失败: %v", err)
	}

	out := decodeStrippedThumb(raw)
	if out == nil {
		t.Fatal("真实样本不应返回 nil")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("还原后的数据不是合法 JPEG: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("期望 jpeg 格式，实际 %s", format)
	}
	// 样本声明宽 0x0a=10、高 0x0a=10
	if cfg.Width != 10 || cfg.Height != 10 {
		t.Errorf("尺寸解析错误：期望 10x10，实际 %dx%d", cfg.Width, cfg.Height)
	}
}

func TestStrippedThumbDataURI(t *testing.T) {
	t.Run("合法数据产生 data URI", func(t *testing.T) {
		uri := strippedThumbDataURI(makeStripped(0x10, 0x10, []byte{0xaa}))
		if !strings.HasPrefix(uri, "data:image/jpeg;base64,") {
			t.Errorf("data URI 前缀错误：%.40s", uri)
		}
		// 载荷部分应为合法 base64
		payload := strings.TrimPrefix(uri, "data:image/jpeg;base64,")
		if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
			t.Errorf("载荷不是合法 base64: %v", err)
		}
	})

	t.Run("非法数据返回空串", func(t *testing.T) {
		if uri := strippedThumbDataURI([]byte{0xff}); uri != "" {
			t.Errorf("非法数据应返回空串，实际 %.40s", uri)
		}
	})
}

func TestExtractPhotoThumbnail(t *testing.T) {
	t.Run("nil photo 返回空串", func(t *testing.T) {
		if got := extractPhotoThumbnail(nil); got != "" {
			t.Errorf("期望空串，实际 %.40s", got)
		}
	})

	t.Run("无缩略图尺寸时返回空串", func(t *testing.T) {
		photo := &tg.Photo{
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoSize{Type: "x", W: 800, H: 600, Size: 12345},
			},
		}
		if got := extractPhotoThumbnail(photo); got != "" {
			t.Errorf("只有普通 PhotoSize 时应返回空串，实际 %.40s", got)
		}
	})

	t.Run("提取 PhotoStrippedSize", func(t *testing.T) {
		photo := &tg.Photo{
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoSize{Type: "x", W: 800, H: 600, Size: 12345},
				&tg.PhotoStrippedSize{Type: "i", Bytes: makeStripped(0x10, 0x10, []byte{0xaa})},
			},
		}
		got := extractPhotoThumbnail(photo)
		if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
			t.Errorf("未能从 PhotoStrippedSize 提取缩略图，得到 %.40s", got)
		}
	})

	t.Run("提取 PhotoCachedSize", func(t *testing.T) {
		photo := &tg.Photo{
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoCachedSize{Type: "s", Bytes: []byte{0xff, 0xd8, 0xff, 0xd9}},
			},
		}
		got := extractPhotoThumbnail(photo)
		if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
			t.Errorf("未能从 PhotoCachedSize 提取缩略图，得到 %.40s", got)
		}
	})

	t.Run("跳过空的 PhotoStrippedSize", func(t *testing.T) {
		photo := &tg.Photo{
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoStrippedSize{Type: "i", Bytes: nil},
				&tg.PhotoCachedSize{Type: "s", Bytes: []byte{0xff, 0xd8, 0xff, 0xd9}},
			},
		}
		got := extractPhotoThumbnail(photo)
		if got == "" {
			t.Error("应跳过空 stripped 尺寸并回退到 cached 尺寸")
		}
	})
}

func TestExtractDocumentThumbnail(t *testing.T) {
	t.Run("nil document 返回空串", func(t *testing.T) {
		if got := extractDocumentThumbnail(nil); got != "" {
			t.Errorf("期望空串，实际 %.40s", got)
		}
	})

	t.Run("无缩略图时返回空串", func(t *testing.T) {
		doc := &tg.Document{ID: 1, MimeType: "application/pdf"}
		if got := extractDocumentThumbnail(doc); got != "" {
			t.Errorf("期望空串，实际 %.40s", got)
		}
	})

	t.Run("提取视频缩略图", func(t *testing.T) {
		doc := &tg.Document{
			ID:       1,
			MimeType: "video/mp4",
			Thumbs: []tg.PhotoSizeClass{
				&tg.PhotoStrippedSize{Type: "i", Bytes: makeStripped(0x20, 0x12, []byte{0xbb})},
			},
		}
		got := extractDocumentThumbnail(doc)
		if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
			t.Errorf("未能从 Document.Thumbs 提取缩略图，得到 %.40s", got)
		}
	})
}

// 缩略图必须足够小，否则塞进消息缓存会撑爆 media_json 列。
func TestThumbnailSizeIsBounded(t *testing.T) {
	// 构造一个偏大的熵数据（真实 stripped 缩略图通常远小于此）
	entropy := bytes.Repeat([]byte{0xab}, 400)
	uri := strippedThumbDataURI(makeStripped(0x28, 0x28, entropy))

	// media_json 列为 8192，单个缩略图应留有充足余量
	if len(uri) > 4096 {
		t.Errorf("缩略图 data URI 过大（%d 字节），可能超出 media_json 容量", len(uri))
	}
}
