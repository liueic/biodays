package mailer

import (
	"bytes"
	htmltemplate "html/template"
	texttemplate "text/template"

	"biodays/server/internal/model"
)

// TemplateData is stored in the outbox payload (sensitive, cleared on send)
// and rendered by the worker at delivery time.
type TemplateData struct {
	Type             string `json:"type"`
	SiteName         string `json:"site_name"`
	SiteURL          string `json:"site_url"`
	FeedbackID       string `json:"feedback_id"`
	Kind             string `json:"kind"`
	KindLabel        string `json:"kind_label"`
	Title            string `json:"title"`
	Nickname         string `json:"nickname"`
	Email            string `json:"email"`
	Content          string `json:"content"`
	SourceURL        string `json:"source_url"`
	StoryID          string `json:"story_id"`
	Status           string `json:"status"`
	StatusLabel      string `json:"status_label"`
	PublicReply      string `json:"public_reply"`
	ConfirmURL       string `json:"confirm_url"`
	UnsubscribeURL   string `json:"unsubscribe_url"`
	TokenHash        string `json:"token_hash"`
	EmailFingerprint string `json:"email_fingerprint"`
	AdminURL         string `json:"admin_url"`
}

var kindLabels = map[string]string{
	model.FeedbackKindTopic:    "想看话题",
	model.FeedbackKindResource: "趣味资源",
	model.FeedbackKindFeature:  "功能建议",
	model.FeedbackKindIssue:    "问题纠错",
}

var statusLabels = map[string]string{
	model.FeedbackStatusPending:   "待处理",
	model.FeedbackStatusReviewing: "处理中",
	model.FeedbackStatusAccepted:  "已采纳",
	model.FeedbackStatusCompleted: "已完成",
	model.FeedbackStatusRejected:  "未采纳",
	model.FeedbackStatusSpam:      "垃圾信息",
}

func KindLabel(kind string) string {
	if label, ok := kindLabels[kind]; ok {
		return label
	}
	return kind
}

func StatusLabel(status string) string {
	if label, ok := statusLabels[status]; ok {
		return label
	}
	return status
}

type Renderer struct {
	SiteName string
	SiteURL  string
}

func NewRenderer(siteName, siteURL string) Renderer {
	if siteName == "" {
		siteName = "biodays"
	}
	return Renderer{SiteName: siteName, SiteURL: siteURL}
}

func (r Renderer) Render(mailType string, data TemplateData) (Message, error) {
	data.Type = mailType
	if data.SiteName == "" {
		data.SiteName = r.SiteName
	}
	if data.SiteURL == "" {
		data.SiteURL = r.SiteURL
	}
	if data.KindLabel == "" && data.Kind != "" {
		data.KindLabel = KindLabel(data.Kind)
	}
	if data.StatusLabel == "" && data.Status != "" {
		data.StatusLabel = StatusLabel(data.Status)
	}

	subject, err := executeText(subjectTemplate, data)
	if err != nil {
		return Message{}, err
	}
	text, err := executeText(textTemplate, data)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := executeHTML(htmlTemplate, data)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Subject: subject,
		HTML:    htmlBody,
		Text:    text,
	}
	if mailType == model.MailTypeUserStatusUpdate && data.UnsubscribeURL != "" {
		msg.Headers = map[string]string{"List-Unsubscribe": "<" + data.UnsubscribeURL + ">"}
	}
	return msg, nil
}

func executeText(source string, data TemplateData) (string, error) {
	tpl, err := texttemplate.New("mail").Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func executeHTML(source string, data TemplateData) (string, error) {
	tpl, err := htmltemplate.New("mail").Parse(source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const subjectTemplate = `{{if eq .Type "admin_new_feedback"}}[{{.SiteName}}] 新反馈 #{{.FeedbackID}} · {{.Title}}{{else if eq .Type "subscription_confirm"}}确认订阅 {{.SiteName}} 反馈 #{{.FeedbackID}} 的处理结果{{else if eq .Type "user_status_update"}}{{.SiteName}} 反馈 #{{.FeedbackID}} 有新的处理结果：{{.StatusLabel}}{{else}}[{{.SiteName}}] SMTP 测试邮件{{end}}`

const textTemplate = `{{if eq .Type "admin_new_feedback"}}{{.SiteName}} 收到一条新反馈。

编号：{{.FeedbackID}}
类型：{{.KindLabel}}
标题：{{.Title}}
称呼：{{if .Nickname}}{{.Nickname}}{{else}}匿名访客{{end}}
邮箱：{{if .Email}}{{.Email}}（{{if eq .Status "pending"}}未验证{{end}}）{{else}}未留{{end}}
关联故事：{{if .StoryID}}{{.StoryID}}{{else}}无{{end}}
资源链接：{{if .SourceURL}}{{.SourceURL}}{{else}}无{{end}}

内容：
{{.Content}}

后台处理：{{.AdminURL}}

—— {{.SiteName}}
{{else if eq .Type "subscription_confirm"}}你好，

有人（可能是你）在 {{.SiteName}} 提交了反馈 #{{.FeedbackID}}，并请求通过邮件接收处理结果。

确认订阅请打开：
{{.ConfirmURL}}

确认链接 24 小时内有效，仅可使用一次。如果不是你提交的，忽略本邮件即可，我们不会再次发送。

—— {{.SiteName}}
{{else if eq .Type "user_status_update"}}你在 {{.SiteName}} 反馈 #{{.FeedbackID}} 的状态更新了。

标题：{{.Title}}
当前状态：{{.StatusLabel}}
处理说明：{{if .PublicReply}}{{.PublicReply}}{{else}}（暂无补充说明）{{end}}

查看或退订：
{{.UnsubscribeURL}}

—— {{.SiteName}}
{{else}}这是一封 {{.SiteName}} SMTP 配置测试邮件。

收到它说明当前 SMTP 主机、加密方式与认证配置可以正常投递。邮件仅会发往配置的管理员接收地址。

—— {{.SiteName}}
{{end}}`

const htmlTemplate = `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><title>{{.SiteName}}</title></head>
<body style="margin:0;padding:24px;background:#c0c0c0;font-family:system-ui,-apple-system,'Segoe UI',sans-serif;color:#111;">
<div style="max-width:560px;margin:0 auto;background:#ffffff;border:2px solid #808080;padding:20px;">
{{if eq .Type "admin_new_feedback"}}
<h1 style="font-size:18px;margin:0 0 12px;">收到一条新反馈</h1>
<table style="border-collapse:collapse;font-size:14px;width:100%;">
<tr><td style="padding:4px 8px 4px 0;color:#555;">编号</td><td>{{.FeedbackID}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">类型</td><td>{{.KindLabel}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">标题</td><td>{{.Title}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">称呼</td><td>{{if .Nickname}}{{.Nickname}}{{else}}匿名访客{{end}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">邮箱</td><td>{{if .Email}}{{.Email}}{{else}}未留{{end}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">关联故事</td><td>{{if .StoryID}}{{.StoryID}}{{else}}无{{end}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">资源链接</td><td>{{if .SourceURL}}<a href="{{.SourceURL}}" rel="noreferrer">{{.SourceURL}}</a>{{else}}无{{end}}</td></tr>
</table>
<h2 style="font-size:15px;margin:18px 0 8px;">内容</h2>
<pre style="white-space:pre-wrap;word-break:break-word;background:#f4f4f4;border:1px solid #c0c0c0;padding:10px;font-family:inherit;font-size:14px;margin:0;">{{.Content}}</pre>
<p style="margin:18px 0 0;"><a href="{{.AdminURL}}" style="color:#0000ee;">打开后台处理这条反馈</a></p>
{{else if eq .Type "subscription_confirm"}}
<h1 style="font-size:18px;margin:0 0 12px;">确认订阅反馈处理结果</h1>
<p style="font-size:14px;line-height:1.6;">有人（可能是你）在 {{.SiteName}} 提交了反馈 <strong>#{{.FeedbackID}}</strong>，并请求通过邮件接收处理结果。</p>
<p style="font-size:14px;line-height:1.6;">点击下面的按钮完成确认。确认前不会发送任何处理结果。</p>
<p style="margin:20px 0;"><a href="{{.ConfirmURL}}" style="display:inline-block;background:#c0c0c0;border:2px solid;border-color:#ffffff #808080 #808080 #ffffff;color:#000;padding:8px 18px;text-decoration:none;font-size:14px;">确认订阅</a></p>
<p style="font-size:13px;line-height:1.6;color:#555;">确认链接 24 小时内有效，仅可使用一次。如果这不是你提交的，忽略本邮件即可，我们不会再次发送。</p>
<p style="font-size:12px;color:#666;word-break:break-all;">若按钮无法使用，请复制链接：{{.ConfirmURL}}</p>
{{else if eq .Type "user_status_update"}}
<h1 style="font-size:18px;margin:0 0 12px;">反馈状态更新</h1>
<p style="font-size:14px;line-height:1.6;">你在 {{.SiteName}} 反馈 <strong>#{{.FeedbackID}}</strong> 的状态更新了。</p>
<table style="border-collapse:collapse;font-size:14px;width:100%;">
<tr><td style="padding:4px 8px 4px 0;color:#555;vertical-align:top;">标题</td><td>{{.Title}}</td></tr>
<tr><td style="padding:4px 8px 4px 0;color:#555;">当前状态</td><td><strong>{{.StatusLabel}}</strong></td></tr>
</table>
<h2 style="font-size:15px;margin:18px 0 8px;">处理说明</h2>
<div style="white-space:pre-wrap;word-break:break-word;background:#f4f4f4;border:1px solid #c0c0c0;padding:10px;font-size:14px;">{{if .PublicReply}}{{.PublicReply}}{{else}}（暂无补充说明）{{end}}</div>
<p style="margin:18px 0 0;font-size:13px;"><a href="{{.UnsubscribeURL}}" style="color:#0000ee;">退订这条反馈的邮件通知</a></p>
{{else}}
<h1 style="font-size:18px;margin:0 0 12px;">SMTP 测试邮件</h1>
<p style="font-size:14px;line-height:1.6;">这是一封 {{.SiteName}} SMTP 配置测试邮件。</p>
<p style="font-size:14px;line-height:1.6;">收到它说明当前 SMTP 主机、加密方式与认证配置可以正常投递。邮件仅会发往配置的管理员接收地址。</p>
{{end}}
<hr style="border:0;border-top:1px solid #c0c0c0;margin:20px 0 12px;">
<p style="font-size:12px;color:#666;margin:0;">{{.SiteName}} · 实验室科学史上的今天</p>
</div>
</body>
</html>`
