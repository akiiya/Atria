package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseChecksums_Normal(t *testing.T) {
	data := []byte("abc123  atria_linux_amd64.tar.gz\ndef456  atria_linux_arm64.tar.gz\n")
	checksums, err := ParseChecksums(data)
	if err != nil {
		t.Fatalf("解析失败: %s", err)
	}

	if checksums["atria_linux_amd64.tar.gz"] != "abc123" {
		t.Errorf("期望 abc123，实际 %s", checksums["atria_linux_amd64.tar.gz"])
	}
	if checksums["atria_linux_arm64.tar.gz"] != "def456" {
		t.Errorf("期望 def456，实际 %s", checksums["atria_linux_arm64.tar.gz"])
	}
}

func TestParseChecksums_EmptyFile(t *testing.T) {
	data := []byte("")
	_, err := ParseChecksums(data)
	if err == nil {
		t.Error("空文件应该返回错误")
	}
}

func TestParseChecksums_InvalidLinesSkipped(t *testing.T) {
	// 包含无效行和有效行
	data := []byte("invalid line\nabc123  atria_linux_amd64.tar.gz\n")
	checksums, err := ParseChecksums(data)
	if err != nil {
		t.Fatalf("解析失败: %s", err)
	}

	// 有效行应该被解析
	if checksums["atria_linux_amd64.tar.gz"] != "abc123" {
		t.Errorf("有效行应该被解析，实际 %v", checksums)
	}
}

func TestParseChecksums_AllInvalidLines(t *testing.T) {
	// 使用只有一个单词的行，无法被解析为 hash + filename
	data := []byte("singleword\nanotherword\n")
	_, err := ParseChecksums(data)
	if err == nil {
		t.Error("全部无效行应该返回错误")
	}
}

func TestVerifyChecksum_Success(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := []byte("hello world")
	os.WriteFile(filePath, content, 0644)

	// 计算正确的 checksum
	expected, err := ComputeChecksum(filePath)
	if err != nil {
		t.Fatalf("计算 checksum 失败: %s", err)
	}

	if err := VerifyChecksum(filePath, expected); err != nil {
		t.Errorf("校验应该成功: %s", err)
	}
}

func TestVerifyChecksum_Failure(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := []byte("hello world")
	os.WriteFile(filePath, content, 0644)

	err := VerifyChecksum(filePath, "wrong_checksum")
	if err == nil {
		t.Error("错误 checksum 应该失败")
	}
}

func TestVerifyChecksum_FileNotFound(t *testing.T) {
	err := VerifyChecksum("/nonexistent/file", "abc123")
	if err == nil {
		t.Error("不存在的文件应该失败")
	}
}

func TestComputeChecksum_Stable(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := []byte("test content")
	os.WriteFile(filePath, content, 0644)

	hash1, _ := ComputeChecksum(filePath)
	hash2, _ := ComputeChecksum(filePath)

	if hash1 != hash2 {
		t.Error("相同文件的 checksum 应该稳定")
	}
}

func TestComputeChecksum_DifferentFiles(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	hash1, _ := ComputeChecksum(file1)
	hash2, _ := ComputeChecksum(file2)

	if hash1 == hash2 {
		t.Error("不同文件的 checksum 应该不同")
	}
}
