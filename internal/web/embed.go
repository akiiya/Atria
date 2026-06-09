// Package web 提供嵌入式 Web 资源的访问。
package web

import (
	"embed"
	"html/template"
	"io/fs"

	rootweb "github.com/user/atria/web"
)

// PageTemplates 持有预解析的页面模板。
type PageTemplates struct {
	*template.Template
}

// getTemplatesFS 返回嵌入的模板文件系统。
func getTemplatesFS() (embed.FS, error) {
	return rootweb.TemplatesFS()
}

// ParseTemplates 解析所有页面模板。
func ParseTemplates() (*PageTemplates, error) {
	templatesFS, err := getTemplatesFS()
	if err != nil {
		return nil, err
	}

	tmpl, err := template.ParseFS(templatesFS,
		"templates/layout.html",
		"templates/partials/*.html",
		"templates/index.html",
		"templates/init.html",
		"templates/login.html",
		"templates/settings.html",
		"templates/placeholder.html",
		"templates/credentials.html",
		"templates/credential_form.html",
		"templates/accounts.html",
		"templates/account_login.html",
		"templates/account_code.html",
		"templates/account_password.html",
		"templates/account_detail.html",
		"templates/chats.html",
		"templates/chat_detail.html",
		"templates/errors/*.html",
	)
	if err != nil {
		return nil, err
	}

	return &PageTemplates{tmpl}, nil
}

// Static 返回嵌入的静态文件系统。
func Static() (fs.FS, error) {
	return rootweb.Static()
}
