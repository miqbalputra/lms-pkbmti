package main

import (
	"html"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (s *Server) serveUjianOnlinePage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	siteKey := os.Getenv("TURNSTILE_SITE_KEY")
	if siteKey == "" && s.cfg.Env != "production" {
		siteKey = "1x00000000000000000000AA"
	}
	nonce, _ := c.Locals("cspNonce").(string)
	if nonce == "" {
		nonce = uuid.NewString()
	}
	page := strings.NewReplacer(
		"{{TURNSTILE_SITE_KEY}}", html.EscapeString(siteKey),
		"{{CSP_NONCE}}", html.EscapeString(nonce),
	).Replace(ujianOnlineHTML)
	return c.SendString(page)
}

var ujianOnlineHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<meta name="theme-color" content="#1c5d94">
<title>Ujian Online — PKBM Tunas Ilmu</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
<style nonce="{{CSP_NONCE}}">
:root{
  --background:#ffffff;--foreground:#0a0a0a;
  --card:#ffffff;--card-foreground:#0a0a0a;
  --primary:#18181b;--primary-foreground:#fafafa;
  --secondary:#f4f4f5;--secondary-foreground:#18181b;
  --muted:#f4f4f5;--muted-foreground:#71717a;
  --accent:#f4f4f5;--accent-foreground:#18181b;
  --destructive:#ef4444;--destructive-foreground:#fafafa;
  --border:#e4e4e7;--input:#e4e4e7;--ring:#18181b;
  --radius:0.5rem;
  --success:#22c55e;--warning:#f59e0b;
}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:#f4f6fb;color:var(--foreground);min-height:100vh;min-height:100dvh;-webkit-font-smoothing:antialiased;-webkit-tap-highlight-color:transparent;touch-action:manipulation}
.wrap{max-width:none;margin:0 auto;padding:0 24px;padding-left:max(24px,env(safe-area-inset-left));padding-right:max(24px,env(safe-area-inset-right))}
.pre-exam-header{position:relative;isolation:isolate;overflow:hidden;min-height:212px;background:linear-gradient(125deg,#1e78ba 0%,#2e6796 58%,#31577e 100%);color:#fff}
.pre-exam-header:before,.pre-exam-header:after{content:"";position:absolute;z-index:-1;border-radius:999px;background:rgba(255,255,255,.08);transform:rotate(-18deg)}
.pre-exam-header:before{width:420px;height:190px;left:-110px;top:-115px}.pre-exam-header:after{width:600px;height:250px;right:-260px;top:-145px}
.pre-exam-header-inner{display:flex;align-items:center;gap:15px;max-width:1120px;margin:0 auto;padding:28px 24px}
.pre-exam-mark{display:grid;place-items:center;width:64px;height:64px;border:2px solid rgba(255,255,255,.6);border-radius:50%;background:rgba(255,255,255,.12);font-size:22px;font-weight:900;letter-spacing:-.08em;box-shadow:0 6px 18px rgba(15,23,42,.18)}
.pre-exam-name{font-size:26px;font-weight:800;letter-spacing:-.04em}.pre-exam-subtitle{margin-top:5px;color:rgba(255,255,255,.85);font-size:15px;letter-spacing:.02em}
.login-card{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);min-height:620px;border-radius:18px;border-color:#dfe3eb;box-shadow:0 18px 50px rgba(15,23,42,.10);overflow:hidden}
.login-brand{position:relative;display:flex;flex-direction:column;justify-content:space-between;overflow:hidden;padding:52px 48px;background:linear-gradient(135deg,#536dff 0%,#3441ed 56%,#3d42d9 100%);color:#fff}
.login-brand:before,.login-brand:after{content:"";position:absolute;border-radius:999px;background:rgba(255,255,255,.10);filter:blur(2px);pointer-events:none}
.login-brand:before{width:300px;height:300px;left:-160px;top:-130px}.login-brand:after{width:320px;height:320px;right:-170px;bottom:-190px}
.brand-logo,.brand-copy,.brand-footer{position:relative;z-index:1}.brand-logo{display:flex;align-items:center;gap:14px}.brand-mark{display:flex;width:54px;height:54px;align-items:center;justify-content:center;border-radius:18px;background:#fff;color:#4354f5;font-size:24px;font-weight:800;box-shadow:0 8px 18px rgba(15,23,42,.15)}
.brand-name{font-size:22px;font-weight:800;letter-spacing:-.04em}.brand-subtitle{margin-top:3px;font-size:13px;color:rgba(255,255,255,.80);font-weight:600}.brand-copy{margin:auto 0;padding:60px 0 42px}.brand-copy h2{max-width:430px;color:#fff;font-size:42px;line-height:1.12;letter-spacing:-.05em;font-weight:800}.brand-copy p{max-width:470px;margin-top:22px;color:rgba(255,255,255,.82);font-size:16px;line-height:1.7}.brand-protected{display:flex;align-items:center;gap:14px;margin-top:34px}.brand-protected-icon{display:flex;width:44px;height:44px;align-items:center;justify-content:center;border-radius:14px;background:rgba(255,255,255,.15)}.brand-protected strong{display:block;font-size:14px}.brand-protected span{display:block;margin-top:4px;color:rgba(255,255,255,.72);font-size:12px}.brand-footer{color:rgba(255,255,255,.65);font-size:12px;font-weight:600}
.login-form-panel{display:flex;flex-direction:column;justify-content:center;padding:52px 56px;background:#fff}.login-heading{margin-bottom:28px}.login-heading .eyebrow{color:#4354f5;font-size:13px;font-weight:800;letter-spacing:.04em;text-transform:uppercase}.login-heading h2{margin-top:10px;color:#101828;font-size:32px;line-height:1.15;letter-spacing:-.045em;font-weight:800}.login-heading p{margin-top:10px;color:#64748b;font-size:14px;line-height:1.6}.login-content{padding:0}.login-footer{padding:0;margin-top:22px}.login-footer .btn-lg{height:50px}.turnstile-wrap{min-height:70px;margin:4px 0 10px;display:flex;justify-content:center;align-items:center}

.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);box-shadow:0 1px 2px 0 rgb(0 0 0 / 0.05);overflow:hidden}
.card-header{padding:24px 24px 0}
.card-content{padding:24px}
.card-footer{padding:0 24px 24px;display:flex;gap:8px}

h1{font-size:18px;font-weight:600;letter-spacing:-0.025em;margin:0}
p.desc{font-size:14px;color:var(--muted-foreground);margin-top:1.5px}

.form-group{margin-bottom:16px}
.form-group:last-child{margin-bottom:0}
label{display:block;font-size:14px;font-weight:500;margin-bottom:6px;color:var(--foreground)}
.input{width:100%;height:44px;padding:0 14px;border:1px solid var(--input);border-radius:var(--radius);font-size:16px;font-family:inherit;background:var(--background);color:var(--foreground);transition:border-color .15s,box-shadow .15s;-webkit-appearance:none;appearance:none}
.input:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}
.input::placeholder{color:var(--muted-foreground)}
.input-mono{font-family:'SF Mono',SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:0.2em;font-size:18px;text-transform:uppercase}

.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;white-space:nowrap;height:44px;padding:0 20px;border-radius:var(--radius);font-size:15px;font-weight:500;font-family:inherit;cursor:pointer;transition:background .15s,border-color .15s,color .15s;border:1px solid transparent;text-decoration:none;-webkit-appearance:none;appearance:none}
.btn:disabled{opacity:.5;pointer-events:none}
.btn:active{transform:scale(0.98)}
.btn-primary{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}
.btn-primary:hover{background:#27272a}
.btn-outline{background:var(--background);color:var(--foreground);border-color:var(--input)}
.btn-outline:hover{background:var(--accent);color:var(--accent-foreground)}
.btn-success{background:var(--success);color:#fff;border-color:var(--success)}
.btn-success:hover{opacity:.9}
.btn-sm{height:36px;padding:0 14px;font-size:13px}
.btn-lg{height:48px;padding:0 24px;font-size:16px}

.error-box{background:#fef2f2;border:1px solid #fecaca;color:var(--destructive);padding:12px 14px;border-radius:var(--radius);font-size:14px;margin-bottom:16px;display:none}
.error-box.show{display:block}

.timer{font-size:28px;font-weight:700;font-variant-numeric:tabular-nums;text-align:center;padding:14px;border-radius:var(--radius);margin-bottom:16px;background:var(--secondary);color:var(--foreground);letter-spacing:-0.025em}
.timer.warning{color:var(--warning);background:#fefce8;border:1px solid #fef08a}
.timer.danger{color:var(--destructive);background:#fef2f2;border:1px solid #fecaca}

.progress{height:6px;background:var(--secondary);border-radius:3px;margin-bottom:16px;overflow:hidden}
.progress-bar{height:100%;background:var(--primary);border-radius:3px;transition:width .3s}

.badge{display:inline-flex;align-items:center;border-radius:9999px;padding:2px 8px;font-size:11px;font-weight:600;line-height:1}
.badge-secondary{background:var(--secondary);color:var(--secondary-foreground);border:1px solid var(--border)}
.badge-success{background:#dcfce7;color:#166534}

.exam-item{border:1px solid var(--border);border-radius:var(--radius);padding:16px;margin-bottom:8px;transition:border-color .15s,box-shadow .15s}
.exam-item h3{font-size:15px;font-weight:600;margin:0 0 6px}
.exam-meta{font-size:13px;color:var(--muted-foreground);display:flex;flex-wrap:wrap;gap:4px 12px;margin:0 0 14px}
.exam-actions{display:flex;align-items:center;gap:8px}
.list-hero{position:relative;overflow:hidden;padding:28px 24px;background:linear-gradient(105deg,#1c5d94,#287db7 52%,#24527d);color:#fff}.list-hero:after{content:'';position:absolute;width:340px;height:340px;right:-150px;top:-210px;border-radius:999px;background:rgba(255,255,255,.08)}.list-hero>*{position:relative;z-index:1}.list-hero h1{font-size:25px;font-weight:800}.list-hero p{margin-top:6px;color:rgba(255,255,255,.76);font-size:13px}.list-content{padding:20px 24px}.list-footer{display:flex;justify-content:space-between;gap:10px;padding:0 24px 24px}

.question-card{border:1px solid var(--border);border-radius:var(--radius);padding:20px 16px;margin-bottom:8px}
.question-num{font-size:12px;font-weight:600;color:var(--muted-foreground);text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px}
.question-text{font-size:15px;line-height:1.7;margin-bottom:16px;color:var(--foreground);word-break:break-word}

/* Candidate workspace: the visual language follows the assessment reference
   (blue institutional header, split stimulus/answer workspace, palette), while
   keeping original Tunas Ilmu branding and content. */
.exam-shell{max-width:1480px;margin:0 auto;border:0;border-radius:18px;background:#fff;box-shadow:0 12px 40px rgba(15,23,42,.12);overflow:hidden}
.exam-topbar{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px 22px;background:linear-gradient(100deg,#1c5d94,#287db7 52%,#24527d);color:#fff}
.exam-brand{display:flex;align-items:center;gap:11px;min-width:0}.exam-brand-mark{display:grid;place-items:center;width:42px;height:42px;border-radius:13px;background:rgba(255,255,255,.16);font-size:17px;font-weight:900;letter-spacing:-.08em;box-shadow:inset 0 0 0 1px rgba(255,255,255,.25)}.exam-brand small{display:block;color:rgba(255,255,255,.7);font-size:10px;font-weight:800;letter-spacing:.15em;text-transform:uppercase}.exam-brand strong{display:block;max-width:44vw;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:16px}.exam-user{display:flex;align-items:center;gap:10px;white-space:nowrap;font-size:12px;color:rgba(255,255,255,.82)}
.exam-timer{display:flex;align-items:center;gap:7px;padding:9px 13px;border-radius:11px;background:rgba(255,255,255,.15);font-family:'SF Mono',SFMono-Regular,Menlo,monospace;font-size:18px;font-weight:800;font-variant-numeric:tabular-nums}.exam-timer.warning{background:#f59e0b;color:#fff}.exam-timer.danger{background:#dc2626;color:#fff}
.exam-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:15px 22px 12px;border-bottom:1px solid #e2e8f0}.exam-title-block h1{font-size:18px;font-weight:800;color:#172033}.exam-title-block p{margin-top:3px;color:#64748b;font-size:12px}.exam-toolbar-actions{display:flex;align-items:center;gap:7px;flex-wrap:wrap}.exam-toolbar-actions .btn{height:36px;padding:0 12px;font-size:12px}.font-controls{display:flex;align-items:center;gap:4px;padding:3px;border:1px solid #dbe3ed;border-radius:10px;background:#f8fafc}.font-controls .btn{height:30px;min-width:30px;padding:0 8px;border:0;background:transparent;color:#334155}.font-controls .btn.active,.font-controls .btn:hover{background:#dceffc;color:#165b8d}
.exam-status{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:10px 22px;background:#f8fbff;color:#64748b;font-size:12px}.exam-status strong{color:#1d6fa8}.exam-save{display:flex;align-items:center;gap:6px}.exam-online{display:flex;align-items:center;gap:6px;color:#15803d}.exam-online.offline{color:#b45309}.exam-progress{height:5px;background:#e8f0f7;overflow:hidden}.exam-progress-bar{height:100%;background:#2f8cca;transition:width .25s}
.exam-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr) 238px;gap:12px;padding:14px 18px 18px;background:#f1f5f9}.exam-panel{min-width:0;min-height:540px;border:1px solid #dbe3ed;border-radius:13px;background:#fff;overflow:hidden;box-shadow:0 1px 3px rgba(15,23,42,.05)}.exam-panel-head{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:13px 15px;border-bottom:1px solid #e5ebf2;background:#f8fafc}.exam-panel-head strong{display:block;color:#1d6fa8;font-size:11px;letter-spacing:.13em;text-transform:uppercase}.exam-panel-head span{display:block;margin-top:3px;color:#1e293b;font-size:14px;font-weight:700}.exam-panel-head small{color:#64748b;font-size:11px}.stimulus-scroll{height:calc(100vh - 310px);min-height:420px;overflow:auto;padding:22px;font-size:16px;line-height:1.75}.stimulus-empty{display:grid;place-items:center;min-height:340px;border:1px dashed #cbd5e1;border-radius:12px;background:#f8fafc;color:#64748b;text-align:center;padding:24px}.stimulus-empty strong{display:block;margin-top:10px;color:#475569}.question-scroll{height:calc(100vh - 310px);min-height:420px;overflow:auto;padding:22px}.question-scroll .question-card{border:0;padding:0;margin:0}.question-scroll .question-text{font-size:18px;font-weight:700;line-height:1.65}.question-footer{display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;padding:14px 22px;border-top:1px solid #e5ebf2;background:#fff}.question-footer .btn{min-height:42px}.question-palette{height:max-content;border:1px solid #dbe3ed;border-radius:13px;background:#fff;padding:15px;box-shadow:0 1px 3px rgba(15,23,42,.05)}.question-palette h2{font-size:14px;font-weight:800;color:#172033}.question-palette p{margin-top:3px;color:#64748b;font-size:11px}.palette-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:7px;margin-top:15px}.palette-grid button{position:relative;height:38px;border:1px solid #dbe3ed;border-radius:8px;background:#f1f5f9;color:#334155;font-size:12px;font-weight:800;cursor:pointer}.palette-grid button:hover{background:#e2e8f0}.palette-grid button.current{border-color:#1d6fa8;background:#1d6fa8;color:#fff}.palette-grid button.answered{border-color:#86efac;background:#dcfce7;color:#166534}.palette-grid button.flagged:after{content:'⚑';position:absolute;right:-5px;top:-10px;color:#d97706;font-size:14px}.palette-legend{display:grid;gap:5px;margin-top:14px;color:#64748b;font-size:11px}.palette-legend span{display:flex;align-items:center;gap:6px}.legend-dot{width:8px;height:8px;border-radius:50%;background:#1d6fa8}.legend-dot.answered{background:#22c55e}.legend-dot.empty{background:#cbd5e1}.palette-submit{width:100%;margin-top:17px}
.exam-panel-head span.question-section{display:block;margin-top:6px;color:#1d6fa8;font-size:12px;font-weight:600;line-height:1.5}
.exam-panel-head span.question-section[hidden]{display:none}
.exam-font-small{font-size:15px}.exam-font-medium{font-size:17px}.exam-font-large{font-size:20px}.exam-font-small .question-text{font-size:16px}.exam-font-large .question-text{font-size:21px}.exam-font-small .option-label{font-size:14px}.exam-font-large .option-label{font-size:18px}
.modal-backdrop{position:fixed;inset:0;z-index:10020;display:flex;align-items:center;justify-content:center;padding:18px;background:rgba(15,23,42,.52);backdrop-filter:blur(4px)}.modal-card{width:min(520px,100%);max-height:90vh;overflow:auto;border-radius:15px;background:#fff;box-shadow:0 22px 60px rgba(15,23,42,.25)}.modal-head{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:16px 18px;border-bottom:1px solid #e5ebf2}.modal-head h2{font-size:17px;font-weight:800}.modal-body{padding:18px;color:#475569;font-size:14px;line-height:1.65}.modal-body ul{padding-left:20px;list-style:disc}.modal-close{height:34px;width:34px;padding:0;border:0;background:transparent;color:#64748b;cursor:pointer;font-size:20px}.modal-close:hover{background:#f1f5f9;border-radius:9px}

.option{display:flex;align-items:flex-start;gap:12px;padding:14px;border:1px solid var(--border);border-radius:var(--radius);margin-bottom:8px;cursor:pointer;transition:all .15s;background:var(--background);min-height:48px}
.option:hover{border-color:#a1a1aa;background:var(--secondary)}
.option.selected{border-color:var(--primary);background:var(--secondary);box-shadow:0 0 0 1px var(--primary)}
.option input[type="radio"]{width:18px;height:18px;margin:1px 0 0 0;accent-color:var(--primary);flex-shrink:0}
.option-label{font-size:15px;line-height:1.5}.select-answer-label{display:block;margin:18px 0 7px;font-size:14px;font-weight:600}
.config-table{width:100%;border-collapse:collapse}.config-table th,.config-table td{padding:10px;border:1px solid #dbe3ed;text-align:left;vertical-align:middle}.config-table thead{background:#f8fafc}.config-table td{text-align:center}.config-table input{width:20px;height:20px;accent-color:#1d6fa8}.table-responsive{width:100%;overflow-x:auto;border-radius:10px}.match-answer{display:grid;gap:10px}.match-row{display:grid;grid-template-columns:minmax(100px,1fr) minmax(150px,1fr);align-items:center;gap:12px;padding:10px;border:1px solid #dbe3ed;border-radius:10px}.scale-labels{display:flex;justify-content:space-between;gap:12px;color:#64748b;font-size:12px}.scale-options{display:flex;flex-wrap:wrap;gap:8px;margin-top:10px}.scale-option{align-items:center;min-width:48px;margin:0;padding:10px}.scale-option input{accent-color:#1d6fa8}.order-answer{display:grid;gap:8px}.order-item{align-items:center}.order-item.dragging{opacity:.5}.order-text{flex:1;min-width:0}.drag-handle{color:#64748b;font-size:18px;cursor:grab}.configured-answer .input{min-height:44px;padding:10px 12px;border:1px solid #cbd5e1;border-radius:8px;background:#fff}.configured-answer .muted{margin:0 0 8px;color:#64748b;font-size:13px}

.textarea{width:100%;min-height:140px;padding:14px;border:1px solid var(--input);border-radius:var(--radius);font-size:16px;font-family:inherit;resize:vertical;background:var(--background);color:var(--foreground);transition:border-color .15s,box-shadow .15s;-webkit-appearance:none;appearance:none}
.textarea:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}

.nav-buttons{display:flex;gap:8px;margin-top:16px}

.result-box{text-align:center;padding:32px 16px}
.result-score{font-size:56px;font-weight:700;color:var(--foreground);letter-spacing:-0.025em;line-height:1}
.result-label{font-size:14px;color:var(--muted-foreground);margin-top:4px}
.result-detail{margin-top:20px;font-size:15px;color:var(--muted-foreground);line-height:1.8}
.result-detail strong{color:var(--foreground);font-weight:600}

.hidden{display:none!important}
.login-card .form-group{margin-bottom:18px}
.login-card .input{height:48px;border-radius:10px}
.login-card .btn-primary{background:#4354f5;border-color:#4354f5}.login-card .btn-primary:hover{background:#3441ed}
/* The pre-exam screens intentionally mirror the calm, centered assessment
   flow from the reference: a full-width institutional banner and one clear
   white card below it. */
.pre-exam-card{display:grid!important;grid-template-columns:minmax(0,1fr)!important;min-height:0!important;max-width:676px!important;margin:-45px auto 32px!important;position:relative;z-index:2;border-radius:15px!important;box-shadow:0 20px 46px rgba(15,23,42,.18)!important}
.pre-exam-card .login-brand{display:none}.pre-exam-card .login-form-panel{min-width:0!important;width:100%!important;padding:48px 70px 44px}.pre-exam-card .login-content,.pre-exam-card .login-footer{min-width:0!important;width:100%!important}.pre-exam-card .login-footer .btn{box-sizing:border-box;max-width:100%;min-width:0;width:100%}.pre-exam-card .login-heading{margin-bottom:25px}.pre-exam-card .login-heading .eyebrow{color:#1d6fa8}.pre-exam-card .login-heading h2{font-size:28px}.pre-exam-card .login-heading p{font-size:15px}.pre-exam-card .card-footer{padding:0;margin-top:20px}.pre-exam-card .btn-primary{background:#147de1;border-color:#147de1;border-radius:8px}.pre-exam-card .btn-primary:hover{background:#0e6fc9}
.pre-exam-card .login-heading{text-align:center}.pre-exam-card .login-heading h2{letter-spacing:-.035em}.pre-exam-card .login-heading p{max-width:430px;margin-left:auto;margin-right:auto}.login-field{position:relative}.login-field .field-icon{position:absolute;left:13px;bottom:14px;z-index:1;display:grid;place-items:center;width:22px;height:22px;color:#64748b;font-size:17px;line-height:1}.login-field .input{padding-left:46px}.pre-exam-card .input{border-color:#d6dde8}.pre-exam-card .input:focus{border-color:#147de1;box-shadow:0 0 0 3px rgba(20,125,225,.13)}
.pre-exam-list{max-width:780px!important;margin:-45px auto 32px!important;position:relative;z-index:2;border-radius:15px!important;box-shadow:0 20px 46px rgba(15,23,42,.18)!important}
.pre-exam-list .list-hero{padding:25px 30px;background:#fff;color:#172033;border-bottom:1px solid #e5ebf2}.pre-exam-list .list-hero:after{background:rgba(29,111,168,.06)}.pre-exam-list .list-hero h1{color:#172033}.pre-exam-list .list-hero p{color:#64748b}.pre-exam-list .list-content{padding:22px 30px}.pre-exam-list .list-footer{padding:0 30px 26px}.pre-exam-list .exam-item{border-radius:10px}.pre-exam-list .exam-item .btn-primary{background:#147de1;border-color:#147de1}

.offline-overlay{position:fixed;inset:0;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;z-index:9999;backdrop-filter:blur(4px);-webkit-backdrop-filter:blur(4px)}
.offline-box{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);padding:32px 24px;max-width:360px;width:90%;text-align:center;box-shadow:0 8px 30px rgba(0,0,0,.15)}
.offline-icon{width:56px;height:56px;margin:0 auto 16px;background:#fef2f2;border-radius:50%;display:flex;align-items:center;justify-content:center}
.offline-icon svg{width:28px;height:28px;color:var(--destructive)}
.offline-title{font-size:17px;font-weight:600;margin:0 0 8px;color:var(--foreground)}
.offline-desc{font-size:14px;color:var(--muted-foreground);margin:0 0 20px;line-height:1.5}
.offline-status{display:flex;align-items:center;justify-content:center;gap:8px;font-size:13px;color:var(--muted-foreground);margin-bottom:16px}
.offline-dot{width:8px;height:8px;border-radius:50%;background:var(--destructive);animation:pulse 1.5s ease-in-out infinite}
.offline-dot.online{background:var(--success)}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}

@media(max-width:640px){
  .wrap{padding:12px 12px;padding:12px max(12px,env(safe-area-inset-left))}
  .card-header{padding:20px 16px 0}
  .card-content{padding:20px 16px}
  .card-footer{padding:0 16px 20px}
  h1{font-size:16px}
  .timer{font-size:24px;padding:12px}
  .result-score{font-size:48px}
  .nav-buttons{flex-wrap:wrap}
  .nav-buttons .btn{flex:1 1 calc(50% - 4px);min-width:0}
  .nav-buttons .btn:first-child{flex:1 1 100%}
  .pre-exam-header{min-height:154px}.pre-exam-header-inner{padding:22px 18px}.pre-exam-mark{width:48px;height:48px;font-size:17px}.pre-exam-name{font-size:20px}.pre-exam-subtitle{font-size:12px}.pre-exam-card{margin:-28px auto 18px!important}.pre-exam-card .login-form-panel{padding:34px 24px 30px}.pre-exam-card .login-heading h2{font-size:25px}.pre-exam-list{margin:-28px auto 18px!important}.pre-exam-list .list-hero,.pre-exam-list .list-content{padding:20px}.pre-exam-list .list-footer{padding:0 20px 20px}
  .login-card{grid-template-columns:1fr;min-height:0}.login-brand{min-height:270px;padding:30px 28px}.brand-copy{padding:34px 0 20px}.brand-copy h2{font-size:30px}.brand-copy p{font-size:14px;margin-top:14px}.brand-protected{margin-top:20px}.brand-footer{display:none}.login-form-panel{padding:34px 28px 38px}.login-heading h2{font-size:27px}
  .exam-topbar{padding:12px 13px}.exam-brand-mark{width:36px;height:36px;border-radius:11px;font-size:15px}.exam-brand strong{max-width:48vw;font-size:14px}.exam-brand small{font-size:8px}.exam-user{gap:6px}.exam-user span{display:none}.exam-timer{padding:8px 9px;font-size:15px}.exam-toolbar{padding:12px 13px}.exam-title-block h1{font-size:16px}.exam-toolbar-actions{width:100%;justify-content:space-between}.exam-status{padding:9px 13px}.exam-grid{display:block;padding:10px;background:#f1f5f9}.exam-panel{min-height:0;margin-bottom:10px}.stimulus-scroll,.question-scroll{height:auto;min-height:0;max-height:none;padding:17px}.question-panel{display:flex;flex-direction:column}.question-footer{padding:11px 13px}.question-footer .btn{flex:1 1 calc(50% - 4px)}.question-footer .btn:last-child{flex-basis:100%}.question-palette{display:none}.exam-panel-head{padding:11px 13px}.exam-status{font-size:11px}.exam-font-large .question-text{font-size:20px}
  .match-row{grid-template-columns:minmax(0,1fr)}.config-table th,.config-table td{padding:8px;font-size:13px}.scale-option{padding:9px;min-width:44px}
}
@media(max-width:360px){
  .wrap{padding:8px 8px;padding:8px max(8px,env(safe-area-inset-left))}
  .card-header{padding:16px 12px 0}
  .card-content{padding:16px 12px}
  .card-footer{padding:0 12px 16px}
  .input{height:40px;font-size:14px}
  .input-mono{font-size:16px}
  .btn{height:40px;font-size:14px;padding:0 16px}
  .btn-lg{height:44px}
  .option{padding:12px;min-height:44px}
  .question-card{padding:16px 12px}
  .login-brand{padding:24px 20px;min-height:238px}.brand-mark{width:46px;height:46px;border-radius:15px;font-size:20px}.brand-name{font-size:18px}.brand-subtitle{font-size:11px}.brand-copy{padding:28px 0 12px}.brand-copy h2{font-size:26px}.brand-copy p{font-size:13px;line-height:1.5}.brand-protected{margin-top:16px}.login-form-panel{padding:28px 20px 30px}.login-heading{margin-bottom:22px}.login-heading h2{font-size:25px}
}
/* Keep every primary control comfortably tappable, including the compact
   login breakpoint and small utility buttons. */
.btn,.input{min-height:44px}
.exam-toolbar-actions .btn,.font-controls .btn,.palette-grid button,.modal-close,.question-footer .btn{min-height:44px}
.font-controls .btn{height:44px;min-width:44px}
.palette-grid{grid-template-columns:repeat(4,minmax(44px,1fr))}
.palette-grid button{height:44px;min-width:44px}
.modal-close{height:44px;width:44px}
</style>
</head>
<body>
<div id="preExamHeader" class="pre-exam-header">
  <div class="pre-exam-header-inner"><div class="pre-exam-mark">TI</div><div><div class="pre-exam-name">Tunas Ilmu</div><div class="pre-exam-subtitle">Portal Ujian Online</div></div></div>
</div>
<div class="wrap">

<!-- Login -->
<div id="loginCard" class="login-card card pre-exam-card">
  <div class="login-brand">
    <div class="brand-logo"><div class="brand-mark">TI</div><div><div class="brand-name">Tunas Ilmu Learn</div><div class="brand-subtitle">PKBM Tunas Ilmu</div></div></div>
    <div class="brand-copy"><h2>Ujian Online Peserta Didik.</h2><p>Kerjakan ujian dengan nyaman melalui platform pembelajaran Tunas Ilmu Learn yang terhubung dengan sekolah.</p><div class="brand-protected"><div class="brand-protected-icon"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 3 5 6v5c0 4.5 2.9 8.5 7 10 4.1-1.5 7-5.5 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg></div><div><strong>Akses Terlindungi</strong><span>Verifikasi keamanan untuk setiap sesi masuk.</span></div></div></div>
    <div class="brand-footer">© 2026 PKBM Tunas Ilmu • Tunas Ilmu Learn</div>
  </div>
  <div class="login-form-panel">
    <div class="login-heading"><span class="eyebrow">Ujian Online</span><h2>Masuk ke Ujian Online</h2><p>Masukkan NISN dan kode akses dari guru Anda untuk melanjutkan.</p></div>
  <div class="card-content login-content">
    <div id="loginError" class="error-box"></div>
    <div class="form-group login-field">
      <span class="field-icon" aria-hidden="true">◉</span>
      <label for="nisn">NISN</label>
      <input class="input" type="text" id="nisn" placeholder="Nomor Induk Siswa Nasional" maxlength="20" autocomplete="off">
    </div>
    <div class="form-group login-field">
      <span class="field-icon" aria-hidden="true">▣</span>
      <label for="aksesKode">Kode Akses</label>
      <input class="input input-mono" type="text" id="aksesKode" placeholder="XXXXXX" maxlength="6" autocomplete="off" style="text-transform:uppercase">
    </div>
    <div class="form-group turnstile-wrap" id="turnstileContainer">
      <div class="cf-turnstile" data-sitekey="{{TURNSTILE_SITE_KEY}}" data-theme="light" data-callback="onTurnstileSuccess" data-expired-callback="onTurnstileExpired" data-error-callback="onTurnstileError"></div>
    </div>
  </div>
  <div class="card-footer login-footer">
    <button class="btn btn-primary btn-lg" data-action="check-exam" id="cekBtn" style="width:100%">Masuk & Cari Ujian</button>
  </div>
  </div>
</div>

<!-- Exam List -->
<div id="listCard" class="card hidden pre-exam-list">
  <div class="list-hero"><h1>Konfirmasi ujian</h1><p>Silakan pilih ujian yang ditugaskan untuk Anda. NISN: <strong id="displayNisn"></strong></p></div>
  <div class="list-content">
    <div id="examList"></div>
  </div>
  <div class="list-footer">
    <button class="btn btn-outline" data-action="show-login" style="width:100%">Ganti Akun</button>
  </div>
</div>

<!-- Exam Taking -->
<div id="examCard" class="exam-shell hidden">
  <header class="exam-topbar">
    <div class="exam-brand"><div class="exam-brand-mark">TI</div><div><small>Tunas Ilmu Learn</small><strong id="examTitle">Ujian Online</strong></div></div>
    <div class="exam-user"><span>Peserta didik</span><div class="exam-timer" id="timer" role="timer" aria-live="off">00:00:00</div></div>
  </header>
  <div class="exam-toolbar">
  <div class="exam-title-block"><h1 id="examSubject">Ruang ujian</h1><p><span id="soalBadge">Soal 1/0</span> · <span id="answeredSummary">0 dari 0 soal terjawab</span></p></div>
    <div class="exam-toolbar-actions">
      <div class="font-controls" aria-label="Ukuran teks"><button class="btn active" data-action="font-medium" aria-label="Ukuran teks sedang">A</button><button class="btn" data-action="font-small" aria-label="Ukuran teks kecil">A<sup>−</sup></button><button class="btn" data-action="font-large" aria-label="Ukuran teks besar">A<sup>+</sup></button></div>
      <button class="btn btn-outline hidden" data-action="exit-response-edit" id="exitResponseEditBtn">Kembali ke hasil</button><button class="btn btn-outline" data-action="open-info">ⓘ Informasi soal</button><button class="btn btn-outline" data-action="open-palette">▦ Daftar soal</button><button class="btn btn-outline" data-action="flag-question" id="flagBtn">⚑ Ragu-ragu</button>
    </div>
  </div>
  <div class="exam-status"><span class="exam-save" id="examSaveStatus" aria-live="polite">✓ Jawaban siap disimpan</span><span class="exam-online" id="examOnlineStatus">● Terhubung</span></div>
  <div class="exam-progress" id="examProgress" role="progressbar" aria-label="Kemajuan jawaban" aria-valuemin="0" aria-valuemax="100" aria-valuenow="0" aria-valuetext="0 dari 0 soal terjawab"><div class="exam-progress-bar" id="progressBar" style="width:0%"></div></div>
  <div class="exam-grid">
    <section class="exam-panel stimulus-panel"><div class="exam-panel-head"><div><strong>Stimulus</strong><span>Bacaan / informasi pendukung</span></div><small>Panel dapat digulir</small></div><div class="stimulus-scroll" id="stimulusContainer"></div></section>
    <section class="exam-panel question-panel"><div class="exam-panel-head"><div><strong id="questionLabel">Pertanyaan 1</strong><span id="questionSection" class="question-section" hidden></span><span>Pilih atau tulis jawabanmu dengan teliti.</span></div><small id="questionState">Belum dijawab</small></div><div class="question-scroll" id="soalContainer"></div><div class="question-footer"><button class="btn btn-outline" data-action="previous-question" id="prevBtn" disabled>‹ Sebelumnya</button><button class="btn btn-primary" data-action="next-question" id="nextBtn">Berikutnya ›</button><button class="btn btn-success hidden" data-action="finish-exam" id="selesaiBtn">✓ Kirim jawaban</button></div></section>
    <aside class="question-palette"><h2>Daftar soal</h2><p>Pilih nomor untuk berpindah.</p><div class="palette-grid" id="paletteGrid"></div><div class="palette-legend"><span><i class="legend-dot"></i>Sedang dibuka</span><span><i class="legend-dot answered"></i>Sudah dijawab</span><span><i class="legend-dot empty"></i>Belum dijawab</span></div><button class="btn btn-outline palette-submit" data-action="finish-exam">Selesai &amp; kirim</button></aside>
  </div>
</div>

<div id="infoModal" class="modal-backdrop hidden" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="infoModalTitle"><div class="modal-head"><h2 id="infoModalTitle">Informasi pengerjaan</h2><button class="modal-close" data-action="close-modal" aria-label="Tutup">×</button></div><div class="modal-body"><p>Jawaban tersimpan otomatis saat Anda memilih atau mengetik jawaban.</p><ul><li>Gunakan tombol <strong>Ragu-ragu</strong> untuk menandai soal yang ingin diperiksa kembali.</li><li>Gunakan tombol ukuran teks agar nyaman dibaca.</li><li>Daftar soal membantu berpindah langsung ke nomor tertentu.</li><li>Pengiriman tidak dapat dibatalkan setelah dikonfirmasi.</li></ul></div></section></div>
<div id="paletteModal" class="modal-backdrop hidden" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="paletteModalTitle"><div class="modal-head"><h2 id="paletteModalTitle">Daftar soal</h2><button class="modal-close" data-action="close-modal" aria-label="Tutup">×</button></div><div class="modal-body"><div class="palette-grid" id="paletteGridMobile"></div><div class="palette-legend"><span><i class="legend-dot"></i>Sedang dibuka</span><span><i class="legend-dot answered"></i>Sudah dijawab</span><span><i class="legend-dot empty"></i>Belum dijawab</span></div></div></section></div>
<div id="confirmExamModal" class="modal-backdrop hidden" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="confirmExamTitle"><div class="modal-head"><h2 id="confirmExamTitle">Konfirmasi ujian</h2><button class="modal-close" data-action="close-modal" aria-label="Tutup">×</button></div><div class="modal-body"><h3 id="confirmExamName" style="font-size:20px;font-weight:800;color:#172033"></h3><div style="display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:16px 0"><div style="padding:12px;border-radius:10px;background:#f1f5f9;text-align:center"><strong id="confirmExamDuration" style="display:block;font-size:20px;color:#1d6fa8"></strong><span style="font-size:11px">menit</span></div><div style="padding:12px;border-radius:10px;background:#f1f5f9;text-align:center"><strong id="confirmExamStart" style="display:block;font-size:12px;color:#1d6fa8"></strong><span style="font-size:11px">mulai</span></div><div style="padding:12px;border-radius:10px;background:#f1f5f9;text-align:center"><strong id="confirmExamStatus" style="display:block;font-size:12px;color:#1d6fa8"></strong><span style="font-size:11px">status</span></div></div><p id="confirmExamHint">Waktu ujian mulai dihitung setelah Anda menekan tombol mulai. Pastikan koneksi stabil dan siapkan diri sebelum melanjutkan.</p><div style="display:flex;justify-content:flex-end;gap:8px;margin-top:18px"><button class="btn btn-outline" data-action="close-modal">Kembali</button><button class="btn btn-primary" data-action="confirm-start" id="confirmExamAction">Mulai ujian</button></div></div></section></div>
<div id="confirmFinishModal" class="modal-backdrop hidden" role="presentation"><section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="confirmFinishTitle"><div class="modal-head"><h2 id="confirmFinishTitle">Periksa sebelum mengirim</h2><button class="modal-close" data-action="close-modal" aria-label="Tutup">×</button></div><div class="modal-body"><p>Pastikan semua jawaban sudah sesuai. Setelah dikirim, jawaban tidak dapat diubah kecuali guru mengizinkan revisi.</p><div style="display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:16px 0;text-align:center"><div style="padding:12px;border-radius:10px;background:#ecfdf5"><strong id="finishAnswered" style="display:block;font-size:22px;color:#047857">0</strong><span style="font-size:12px">Terjawab</span></div><div style="padding:12px;border-radius:10px;background:#fff7ed"><strong id="finishFlagged" style="display:block;font-size:22px;color:#c2410c">0</strong><span style="font-size:12px">Ragu-ragu</span></div><div style="padding:12px;border-radius:10px;background:#fef2f2"><strong id="finishEmpty" style="display:block;font-size:22px;color:#b91c1c">0</strong><span style="font-size:12px">Kosong</span></div></div><div style="display:flex;justify-content:flex-end;gap:8px;margin-top:18px"><button class="btn btn-outline" data-action="close-modal">Kembali memeriksa</button><button class="btn btn-primary" data-action="confirm-finish-exam" id="confirmFinishAction">Kirim jawaban</button></div></div></section></div>

<!-- Result -->
<div id="resultCard" class="card hidden">
  <div class="card-header"><h1>Hasil Ujian</h1></div>
  <div class="card-content">
    <div class="result-box">
      <div class="result-score" id="scoreValue">0</div>
      <div class="result-label">Nilai Anda</div>
      <div class="result-detail" id="scoreDetail"></div>
    </div>
  </div>
  <div class="card-footer" style="justify-content:center">
    <button class="btn btn-outline hidden" data-action="edit-responses" id="editResponsesBtn">Perbaiki jawaban</button>
    <button class="btn btn-primary" data-action="show-login">Kembali ke Awal</button>
  </div>
</div>

</div>

<!-- Offline Popup -->
<div id="offlineOverlay" class="offline-overlay hidden">
  <div class="offline-box">
    <div class="offline-icon">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="1" y1="1" x2="23" y2="23"></line><path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55"></path><path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39"></path><path d="M10.71 5.05A16 16 0 0 1 22.56 9"></path><path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88"></path><path d="M8.53 16.11a6 6 0 0 1 6.95 0"></path><line x1="12" y1="20" x2="12.01" y2="20"></line></svg>
    </div>
    <div class="offline-status"><span class="offline-dot" id="offlineDot"></span><span id="offlineStatusText">Memeriksa koneksi...</span></div>
    <p class="offline-title">Akses Internet Terputus</p>
    <p class="offline-desc">Jawaban Anda tersimpan secara lokal. Sambungkan ulang untuk menyinkronkan dengan server.</p>
    <button class="btn btn-primary btn-lg" data-action="reconnect" id="reconnectBtn" style="width:100%">Sambungkan Ulang</button>
  </div>
</div>

<!-- Connection Restored Toast -->
<div id="toastReconnect" style="position:fixed;top:16px;left:50%;transform:translateX(-50%);background:var(--success);color:#fff;padding:12px 20px;border-radius:var(--radius);font-size:14px;font-weight:500;z-index:10000;box-shadow:0 4px 12px rgba(0,0,0,.15);display:none">Koneksi tersambung kembali</div>

<script nonce="{{CSP_NONCE}}">
const API='/api';
let state={nisn:'',aksesKode:'',ujians:[],currentUjian:null,pendingExam:null,ujianPesertaId:'',soal:[],jawaban:{},berkas:{},flagged:{},currentIdx:0,timerInterval:null,sisaWaktu:0,mulai:null,offlineQueue:[],saveTimer:null,syncPromise:null,answerSequence:0,submitting:false,draggedConfiguredOrder:null,revisingResponses:false,bolehEditRespons:false,revisionRequestKeys:{}};
let turnstileToken='';

// --- Connectivity Detection ---
let isOnline=navigator.onLine;
function updateOnlineStatus(){
  const wasOnline=isOnline;isOnline=navigator.onLine;
  const status=document.getElementById('examOnlineStatus');
  if(status){status.textContent=isOnline?'● Terhubung':'● Offline';status.classList.toggle('offline',!isOnline)}
  if(!isOnline){showOfflinePopup()}
  else if(!wasOnline&&isOnline){hideOfflinePopup();onReconnect()}
}
window.addEventListener('online',()=>{document.getElementById('offlineStatusText').textContent='Menyambungkan...';document.getElementById('offlineDot').className='offline-dot online';setTimeout(updateOnlineStatus,500)});
window.addEventListener('offline',updateOnlineStatus);

function showOfflinePopup(){
  document.getElementById('offlineDot').className='offline-dot';
  document.getElementById('offlineStatusText').textContent='Internet terputus';
  show(document.getElementById('offlineOverlay'));
}
function hideOfflinePopup(){hide(document.getElementById('offlineOverlay'));document.getElementById('toastReconnect').style.display='block';setTimeout(()=>{document.getElementById('toastReconnect').style.display='none'},3000)}

function onReconnect(){
  const status=document.getElementById('examSaveStatus');if(status)status.textContent='Menyinkronkan jawaban…';
  void flushAnswerQueue().then(()=>{if(state.currentUjian)return loadSoal(state.currentUjian.id)}).catch(()=>{});
}

async function reconnect(){
  const btn=document.getElementById('reconnectBtn');btn.disabled=true;btn.textContent='Menyambungkan...';
  try{
    // Test connection
    const r=await fetch(API+'/health');if(!r.ok)throw new Error();
    // If in exam, reload soal
    if(state.currentUjian){
      await flushAnswerQueue();
      await loadSoal(state.currentUjian.id);
      hideOfflinePopup();
      document.getElementById('toastReconnect').style.display='block';
      setTimeout(()=>{document.getElementById('toastReconnect').style.display='none'},3000);
    }
  }catch(e){
    document.getElementById('offlineStatusText').textContent='Masih tidak ada koneksi';
    document.getElementById('offlineDot').className='offline-dot';
  }finally{btn.disabled=false;btn.textContent='Sambungkan Ulang'}
}

function show(el){el.classList.remove('hidden')}
function hide(el){el.classList.add('hidden')}
const examDialogIds=['infoModal','paletteModal','confirmExamModal','confirmFinishModal'];let examDialogReturnFocus=null;
function getOpenExamDialog(){return examDialogIds.map(id=>document.getElementById(id)).find(el=>el&&!el.classList.contains('hidden'))||null}
function focusableInExamDialog(dialog){return Array.from(dialog.querySelectorAll('a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])')).filter(el=>el.getClientRects().length>0)}
function focusExamDialog(modal){const dialog=modal.querySelector('[role="dialog"]');const target=focusableInExamDialog(modal)[0]||dialog;if(target instanceof HTMLElement)target.focus({preventScroll:true})}
function openExamDialog(modal){if(!modal)return;const active=document.activeElement;if(active instanceof HTMLElement&&!active.closest('.modal-backdrop'))examDialogReturnFocus=active;examDialogIds.forEach(id=>{const other=document.getElementById(id);if(other&&other!==modal)hide(other)});show(modal);const dialog=modal.querySelector('[role="dialog"]');if(dialog&&!dialog.hasAttribute('tabindex'))dialog.setAttribute('tabindex','-1');focusExamDialog(modal);requestAnimationFrame(()=>focusExamDialog(modal))}
function closeExamDialog(modal,restoreFocus=true){if(!modal)return;hide(modal);if(getOpenExamDialog())return;const target=examDialogReturnFocus;examDialogReturnFocus=null;if(restoreFocus&&target?.isConnected)target.focus()}
function showLogin(){resetTurnstile();clearInterval(state.timerInterval);clearTimeout(state.saveTimer);state={nisn:'',aksesKode:'',ujians:[],currentUjian:null,pendingExam:null,ujianPesertaId:'',soal:[],jawaban:{},berkas:{},flagged:{},currentIdx:0,timerInterval:null,sisaWaktu:0,mulai:null,offlineQueue:[],saveTimer:null,syncPromise:null,answerSequence:0,submitting:false,draggedConfiguredOrder:null,revisingResponses:false,bolehEditRespons:false,revisionRequestKeys:{}};examDialogIds.forEach(id=>hide(document.getElementById(id)));examDialogReturnFocus=null;void fetch(API+'/ujian-online/logout',{method:'POST',credentials:'include'}).catch(()=>{});show(document.getElementById('preExamHeader'));show(document.getElementById('loginCard'));hide(document.getElementById('listCard'));hide(document.getElementById('examCard'));hide(document.getElementById('resultCard'))}
function showError(id,msg){const e=document.getElementById(id);e.textContent=msg;show(e)}
function onTurnstileSuccess(token){turnstileToken=token}
function onTurnstileExpired(){turnstileToken=''}
function onTurnstileError(){turnstileToken=''}
function resetTurnstile(){turnstileToken='';if(window.turnstile)window.turnstile.reset()}

document.addEventListener('click',e=>{
  const el=e.target instanceof Element?e.target.closest('[data-action]'):null;
  if(!el)return;
  switch(el.dataset.action){
    case 'check-exam':void cekUjian();break;
    case 'show-login':showLogin();break;
    case 'previous-question':prevSoal();break;
    case 'next-question':nextSoal();break;
    case 'finish-exam':openFinishConfirmation();break;
    case 'confirm-finish-exam':closeExamDialog(document.getElementById('confirmFinishModal'),false);void selesaiUjian(false);break;
    case 'reconnect':void reconnect();break;
    case 'start-exam':openExamConfirm(el.dataset.id||'');break;
    case 'confirm-start':closeExamDialog(document.getElementById('confirmExamModal'),false);void mulaiUjian(state.pendingExam?.id||'');break;
    case 'edit-responses':void editSubmittedResponses();break;
    case 'exit-response-edit':void exitSubmittedResponseEdit();break;
    case 'answer':e.preventDefault();jawab(el.dataset.id||'',Number(el.dataset.value));break;
    case 'config-number':e.preventDefault();saveConfiguredAnswer(el.dataset.id||'',Number(el.dataset.value));break;
    case 'config-order-move':e.preventDefault();moveConfiguredOrder(el.dataset.id||'',Number(el.dataset.index||0),Number(el.dataset.target||0));break;
    case 'config-file-delete':e.preventDefault();void deleteConfiguredFile(el.dataset.id||'',el.dataset.fileId||'');break;
    case 'open-info':openExamDialog(document.getElementById('infoModal'));break;
    case 'open-palette':renderPalette('paletteGridMobile');openExamDialog(document.getElementById('paletteModal'));break;
    case 'close-modal':closeExamDialog(el.closest('.modal-backdrop'));break;
    case 'font-small':setExamFont('small');break;
    case 'font-medium':setExamFont('medium');break;
    case 'font-large':setExamFont('large');break;
    case 'flag-question':toggleFlag();break;
    case 'palette-question':state.currentIdx=Number(el.dataset.index||0);closeExamDialog(document.getElementById('paletteModal'),false);renderSoal();requestAnimationFrame(()=>{const heading=document.getElementById('questionLabel');if(heading){heading.tabIndex=-1;heading.focus()}});break;
  }
});
document.addEventListener('change',e=>{
  const el=e.target instanceof Element?e.target.closest('[data-change-action]'):null;
  if(!el)return;
  if(el.dataset.changeAction==='select-answer')jawab(el.dataset.id||'',Number(el.value));
  if(el.dataset.changeAction==='checkbox-answer')toggleCheckboxAnswer(el.dataset.id||'',Number(el.value),el.checked);
  if(el.dataset.configAnswer==='upload')void uploadConfiguredFile(el);
  else if(el.dataset.configAnswer&&el.dataset.configAnswer!=='text')handleConfiguredChange(el);
});
document.addEventListener('dragstart',e=>{const el=e.target instanceof Element?e.target.closest('[data-order-drag]'):null;if(!el)return;el.classList.add('dragging');state.draggedConfiguredOrder={id:el.dataset.id,value:el.dataset.value};if(e.dataTransfer)e.dataTransfer.effectAllowed='move'});
document.addEventListener('dragend',e=>{const el=e.target instanceof Element?e.target.closest('[data-order-drag]'):null;if(el)el.classList.remove('dragging')});
document.addEventListener('dragover',e=>{const el=e.target instanceof Element?e.target.closest('[data-order-drag]'):null;if(el)e.preventDefault()});
document.addEventListener('drop',e=>{const el=e.target instanceof Element?e.target.closest('[data-order-drag]'):null;if(!el||!state.draggedConfiguredOrder)return;e.preventDefault();if(state.draggedConfiguredOrder.id===el.dataset.id){const question=state.soal.find(item=>item.id===el.dataset.id);if(question){const answer=configuredAnswer(question)||question.konfigurasi.choices.map(item=>item.id);const from=answer.indexOf(state.draggedConfiguredOrder.value);const to=answer.indexOf(el.dataset.value);if(from>=0&&to>=0&&from!==to){const next=[...answer];const [moved]=next.splice(from,1);next.splice(to,0,moved);saveConfiguredAnswer(question.id,next)}}}state.draggedConfiguredOrder=null});
document.addEventListener('keydown',e=>{const dialog=getOpenExamDialog();if(dialog){const panel=dialog.querySelector('[role="dialog"]');if(e.key==='Escape'){e.preventDefault();closeExamDialog(dialog);return}if(e.key==='Tab'){const focusable=focusableInExamDialog(dialog);if(!focusable.length){e.preventDefault();panel?.focus();return}const first=focusable[0],last=focusable[focusable.length-1];if(e.shiftKey&&(document.activeElement===first||!panel?.contains(document.activeElement))){e.preventDefault();last.focus()}else if(!e.shiftKey&&(document.activeElement===last||!panel?.contains(document.activeElement))){e.preventDefault();first.focus()}return}}if(state.currentUjian&&!document.getElementById('examCard').classList.contains('hidden')&&(e.target===document.body||e.target===document.documentElement)){if(e.key==='ArrowLeft')prevSoal();if(e.key==='ArrowRight')nextSoal();if(e.key.toLowerCase()==='m'){renderPalette('paletteGridMobile');openExamDialog(document.getElementById('paletteModal'))}}});
document.addEventListener('input',e=>{
  const el=e.target instanceof Element?e.target.closest('[data-action="text-answer"]'):null;
  if(el)jawabTeks(el.dataset.id||'',el.value);
  const configured=e.target instanceof Element?e.target.closest('[data-config-answer="text"]'):null;
  if(configured)saveConfiguredAnswer(configured.dataset.id||'',configured.value,false);
});

async function cekUjian(){
const nisn=document.getElementById('nisn').value.trim();
const kode=document.getElementById('aksesKode').value.trim();
if(!nisn||!kode){showError('loginError','NISN dan Kode Akses wajib diisi.');return}
if(!turnstileToken){showError('loginError','Silakan selesaikan verifikasi keamanan terlebih dahulu.');return}
document.getElementById('loginError').classList.remove('show');
document.getElementById('cekBtn').disabled=true;document.getElementById('cekBtn').textContent='Mencari...';
try{
const fd=new FormData();fd.append('nisn',nisn);fd.append('aksesKode',kode);
fd.append('cf-turnstile-response',turnstileToken);
const r=await fetch(API+'/ujian-online/cek',{method:'POST',body:fd,credentials:'include'});
const d=await r.json();
if(!r.ok)throw new Error(d.error||'Gagal');
state.nisn=nisn;state.aksesKode=kode;state.ujians=d;
document.getElementById('displayNisn').textContent=nisn;
renderExamList();
hide(document.getElementById('loginCard'));show(document.getElementById('listCard'));
}catch(e){showError('loginError',e.message);resetTurnstile()}
finally{document.getElementById('cekBtn').disabled=false;document.getElementById('cekBtn').textContent='Cari Ujian'}
}

function renderExamList(){
const c=document.getElementById('examList');
if(!state.ujians.length){c.innerHTML='<p style="color:var(--muted-foreground);font-size:14px;text-align:center;padding:24px 0">Tidak ada ujian aktif untuk kode ini.</p>';return}
c.innerHTML=state.ujians.map(u=>{
const mulai=new Date(u.waktuMulai).toLocaleString('id-ID',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});
const selesai=new Date(u.waktuSelesai).toLocaleString('id-ID',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});
const ongoing=u.status==='mulai';const waitingGrade=u.status==='menunggu_nilai'||(u.status==='dikunci'&&u.skor==null);const finished=u.status==='selesai'||u.status==='dikunci'||waitingGrade||(u.sudahMengerjakan&&!ongoing);const canRevise=finished&&u.bolehEditRespons===true&&u.status!=='dikunci';
let badge=ongoing?'<span class="badge" style="background:#fff7ed;color:#c2410c">Sedang berlangsung</span>':waitingGrade?'<span class="badge" style="background:#eff6ff;color:#1d4ed8">'+(u.status==='dikunci'?'Dikunci · menunggu nilai':'Menunggu penilaian guru')+'</span>':finished?'<span class="badge badge-success">Selesai</span>':'';
 return '<div class="exam-item"><h3>'+esc(u.judul)+'</h3><div class="exam-meta"><span>'+esc(u.mapel?.namaMapel||'')+'</span><span>'+u.durasiMenit+' menit</span><span>'+mulai+' — '+selesai+'</span></div><div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">'+badge+'<button class="btn btn-primary btn-sm" data-action="start-exam" data-id="'+esc(u.id)+'" '+(finished&&!canRevise?'disabled':'')+'>'+((ongoing)?'Lanjutkan ujian':canRevise?'Lihat / perbaiki jawaban':finished?'Sudah dikerjakan':'Lihat instruksi')+'</button></div></div>'
}).join('');
}

function openExamConfirm(ujianId){
 const exam=state.ujians.find(u=>u.id===ujianId);if(!exam)return;
 state.pendingExam=exam;
 document.getElementById('confirmExamName').textContent=exam.judul||'Ujian Online';
 document.getElementById('confirmExamDuration').textContent=(exam.durasiMenit||60);
 document.getElementById('confirmExamStart').textContent=new Date(exam.waktuMulai).toLocaleTimeString('id-ID',{hour:'2-digit',minute:'2-digit'});
 const ongoing=exam.status==='mulai';const canRevise=!ongoing&&exam.bolehEditRespons===true&&exam.status!=='dikunci';document.getElementById('confirmExamStatus').textContent=ongoing?'Sedang berlangsung':canRevise?'Sudah dikirim':'Siap';
 document.getElementById('confirmExamAction').textContent=canRevise?'Perbaiki jawaban':ongoing?'Lanjutkan ujian':'Mulai ujian';
 document.getElementById('confirmExamHint').textContent=canRevise?'Guru mengizinkan revisi. Perubahan tersimpan sebagai riwayat dan tidak mengatur ulang waktu ujian.':ongoing?'Ujian ini sudah dimulai. Anda dapat melanjutkan dari jawaban terakhir yang tersimpan.':'Waktu ujian mulai dihitung setelah Anda menekan tombol mulai. Pastikan koneksi stabil dan siapkan diri sebelum melanjutkan.';
 openExamDialog(document.getElementById('confirmExamModal'));
}

async function mulaiUjian(ujianId){
try{
const r=await fetch(API+'/ujian-online/'+ujianId+'/mulai',{method:'POST',credentials:'include'});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
 state.currentUjian=state.ujians.find(u=>u.id===ujianId);
 state.ujianPesertaId=d.id;
 await loadSoal(ujianId);
 hide(document.getElementById('preExamHeader'));hide(document.getElementById('listCard'));show(document.getElementById('examCard'));
 document.getElementById('examTitle').textContent=state.currentUjian?.judul||'Ujian';
 document.getElementById('examSubject').textContent=state.currentUjian?.mapel?.namaMapel||'Ujian Online';
 const firstQuestionHeading=document.getElementById('questionLabel');if(firstQuestionHeading){firstQuestionHeading.tabIndex=-1;firstQuestionHeading.focus()}
 let savedFont='medium';try{savedFont=localStorage.getItem('ujian-font-scale')||'medium'}catch(_){ }setExamFont(savedFont);
 state.currentUjian.gracePeriodMenit=d.gracePeriodMenit||5;
 if(!state.revisingResponses&&!state.timerInterval&&d.status==='mulai')startTimer(d.mulai,state.sisaWaktu);
}catch(e){alert(e.message)}
}

async function loadSoal(ujianId){
const r=await fetch(API+'/ujian-online/'+ujianId+'/soal',{credentials:'include'});
const d=await r.json();if(!r.ok){
  if(d.error&&d.error.includes('habis')){hide(document.getElementById('examCard'));show(document.getElementById('preExamHeader'));show(document.getElementById('resultCard'));document.getElementById('scoreValue').textContent='—';document.getElementById('scoreDetail').innerHTML='<strong>Waktu ujian sudah habis.</strong>';}
  throw new Error(d.error||'Gagal');
}
const previousIndex=state.currentIdx;const hadQuestions=state.soal.length>0;
state.soal=d.soal||[];state.sisaWaktu=d.sisaWaktu||0;
state.currentUjian=state.currentUjian||{};
state.currentUjian.gracePeriodMenit=d.gracePeriodMenit||5;
state.revisingResponses=Boolean(d.bolehEditRespons);state.bolehEditRespons=Boolean(d.bolehEditRespons);
const exitRevisionButton=document.getElementById('exitResponseEditBtn');if(exitRevisionButton)exitRevisionButton.classList.toggle('hidden',!state.revisingResponses);
const resultRevisionButton=document.getElementById('editResponsesBtn');if(resultRevisionButton)resultRevisionButton.classList.toggle('hidden',!state.bolehEditRespons);
const saveStatus=document.getElementById('examSaveStatus');if(saveStatus&&state.revisingResponses)saveStatus.textContent='Mode revisi · perubahan disimpan dengan riwayat';
state.jawaban={};state.berkas={};
(d.jawaban||[]).forEach(j=>{state.jawaban[j.ujianSoalId]=j.jawaban;state.berkas[j.ujianSoalId]=Array.isArray(j.berkas)?j.berkas:[]});
restoreExamLocalState();
state.currentIdx=hadQuestions?Math.min(previousIndex,Math.max(0,state.soal.length-1)):0;renderSoal();
// Restart timer with server time (handles reconnect / grace period)
if(!state.revisingResponses&&state.sisaWaktu!==0){startTimer(null,state.sisaWaktu)}
}

function examStorageKey(){return state.ujianPesertaId?'ujian-online-state:'+state.ujianPesertaId:''}
function persistExamLocalState(){const key=examStorageKey();if(!key)return;try{sessionStorage.setItem(key,JSON.stringify({jawaban:state.jawaban,flagged:state.flagged,offlineQueue:state.offlineQueue,answerSequence:state.answerSequence}))}catch(_){}}
function restoreExamLocalState(){const key=examStorageKey();if(!key)return;try{const saved=JSON.parse(sessionStorage.getItem(key)||'null');if(!saved)return;state.flagged=saved.flagged&&typeof saved.flagged==='object'?saved.flagged:{};state.offlineQueue=Array.isArray(saved.offlineQueue)?saved.offlineQueue:[];state.answerSequence=Number(saved.answerSequence)||0;state.offlineQueue.forEach(item=>{if(item&&item.soalId)state.jawaban[item.soalId]=item.val})}catch(_){}}
function hasExamAnswer(question,value){if(value===null||value===undefined)return false;if(question&&question.tipe==='checkbox'){try{return JSON.parse(value).length>0}catch(_){return false}}if(question&&question.konfigurasi){try{const parsed=JSON.parse(value);if(Array.isArray(parsed))return parsed.length>0;if(parsed&&typeof parsed==='object')return Object.keys(parsed).length>0;if(typeof parsed==='string')return parsed.trim()!=='';return typeof parsed==='number'||typeof parsed==='boolean'}catch(_){return false}}return Array.isArray(value)?value.length>0:typeof value==='string'?value.trim()!=='':true}

function renderSoal(){
const total=state.soal.length;if(!total)return;
const s=state.soal[state.currentIdx];
document.getElementById('soalBadge').textContent='Soal '+(state.currentIdx+1)+'/'+total;
const answeredCount=state.soal.filter(question=>hasExamAnswer(question,state.jawaban[question.id])).length;
const progressPercent=Math.round(answeredCount/total*100);document.getElementById('progressBar').style.width=progressPercent+'%';
const progress=document.getElementById('examProgress');progress.setAttribute('aria-valuenow',String(progressPercent));progress.setAttribute('aria-valuetext',answeredCount+' dari '+total+' soal terjawab');
document.getElementById('answeredSummary').textContent=answeredCount+' dari '+total+' soal terjawab';
document.getElementById('questionLabel').textContent='Pertanyaan '+(state.currentIdx+1);
const sectionLabel=document.getElementById('questionSection');sectionLabel.hidden=!s.namaBagian;sectionLabel.textContent=s.namaBagian?(s.deskripsiBagian?'Bagian: '+s.namaBagian+' — '+s.deskripsiBagian:'Bagian: '+s.namaBagian):'';
document.getElementById('questionState').textContent=(hasExamAnswer(s,state.jawaban[s.id])?'Sudah dijawab':'Belum dijawab')+(state.revisingResponses?' · Mode revisi':'');
document.getElementById('flagBtn').textContent=(state.flagged&&state.flagged[s.id]?'⚑ Ditandai':'⚑ Ragu-ragu');
document.getElementById('flagBtn').classList.toggle('btn-success',Boolean(state.flagged&&state.flagged[s.id]));
document.getElementById('flagBtn').classList.toggle('btn-outline',!(state.flagged&&state.flagged[s.id]));
const c=document.getElementById('soalContainer');
let html='<div class="question-card"><div class="question-text">'+esc(s.pertanyaan)+'</div>';
if((s.konfigurasi&&Object.keys(s.konfigurasi).length)||['pg_tunggal','pg_kompleks','benar_salah','menjodohkan','isian_singkat','uraian','skala_linear','rating','kisi_pg','kisi_checkbox','tanggal','waktu','susun_urutan'].includes(s.tipe)){html+=renderConfiguredAnswer(s);html+='</div>';c.innerHTML=html;renderStimulus(s);renderPalette('paletteGrid');document.getElementById('prevBtn').disabled=state.currentIdx===0;document.getElementById('nextBtn').classList.toggle('hidden',state.currentIdx>=total-1);document.getElementById('selesaiBtn').classList.toggle('hidden',state.revisingResponses||state.currentIdx<total-1);return}
const displayIndex=i=>Array.isArray(s.opsiIndex)?Number(s.opsiIndex[i]):i;
if((s.tipe==='pg'||s.tipe==='true_false')&&s.opsi){
s.opsi.forEach((op,i)=>{
const value=displayIndex(i);const sel=state.jawaban[s.id]===String(value)?'selected':'';
 html+='<label class="option '+sel+'" data-action="answer" data-id="'+esc(s.id)+'" data-value="'+value+'"><input type="radio" name="soal_'+esc(s.id)+'" value="'+value+'" '+(sel?'checked':'')+'><span class="option-label"><strong>'+String.fromCharCode(65+i)+'</strong>. '+esc(op)+'</span></label>';
});
}else if(s.tipe==='checkbox'&&s.opsi){
 let selected=[];try{const parsed=JSON.parse(state.jawaban[s.id]||'[]');if(Array.isArray(parsed))selected=parsed}catch(_){}
 s.opsi.forEach((op,i)=>{const value=displayIndex(i);const checked=selected.includes(value);html+='<label class="option '+(checked?'selected':'')+'"><input type="checkbox" data-change-action="checkbox-answer" data-id="'+esc(s.id)+'" value="'+value+'" '+(checked?'checked':'')+'><span class="option-label"><strong>'+String.fromCharCode(65+i)+'</strong>. '+esc(op)+'</span></label>'});
}else if(s.tipe==='dropdown'&&s.opsi){
 const current=state.jawaban[s.id]??'';html+='<label class="select-answer-label" for="answer_'+esc(s.id)+'">Pilih satu jawaban</label><select class="input" id="answer_'+esc(s.id)+'" data-change-action="select-answer" data-id="'+esc(s.id)+'"><option value="">Pilih jawaban</option>'+s.opsi.map((op,i)=>{const value=displayIndex(i);return '<option value="'+value+'" '+(current===String(value)?'selected':'')+'>'+esc(op)+'</option>'}).join('')+'</select>';
}else if(s.tipe==='short_answer'){
 html+='<input class="input" type="text" data-action="text-answer" data-id="'+esc(s.id)+'" value="'+esc(state.jawaban[s.id]||'')+'" placeholder="Ketik jawaban singkat Anda..." autocomplete="off">';
}else{
 html+='<textarea class="textarea" data-action="text-answer" data-id="'+esc(s.id)+'" placeholder="Tulis jawaban Anda di sini...">'+esc(state.jawaban[s.id]||'')+'</textarea>';
}
html+='</div>';
c.innerHTML=html;
renderStimulus(s);
renderPalette('paletteGrid');
document.getElementById('prevBtn').disabled=state.currentIdx===0;
document.getElementById('nextBtn').classList.toggle('hidden',state.currentIdx>=total-1);
document.getElementById('selesaiBtn').classList.toggle('hidden',state.revisingResponses||state.currentIdx<total-1);
}

function configuredAnswer(question){try{return JSON.parse(state.jawaban[question.id]||'null')}catch(_){return null}}
function saveConfiguredAnswer(id,value,rerender=true){const answer=JSON.stringify(value);state.jawaban[id]=answer;sendJawaban(id,answer);if(rerender){const active=document.activeElement;const ref=active instanceof HTMLElement&&active.dataset.configAnswer?{kind:active.dataset.configAnswer,id:active.dataset.id||'',key:active.dataset.key||'',value:active.dataset.value||'',name:active.getAttribute('name')||''}:null;renderSoal();if(ref){const target=Array.from(document.querySelectorAll('[data-config-answer]')).find(item=>item.dataset.configAnswer===ref.kind&&item.dataset.id===ref.id&&item.dataset.key===ref.key&&item.dataset.value===ref.value&&(!ref.name||item.getAttribute('name')===ref.name));if(target instanceof HTMLElement)target.focus()}}}
function handleConfiguredChange(el){const id=el.dataset.id||'';const question=state.soal.find(item=>item.id===id);if(!question)return;const config=question.konfigurasi||{};let value=configuredAnswer(question);if(value===null)value={};const kind=el.dataset.configAnswer;const key=el.dataset.key||'';const choice=el.dataset.value||'';
 if(kind==='single'||kind==='select'||kind==='number'){value=kind==='number'?Number(el.value):el.value}
 else if(kind==='multi'){if(!Array.isArray(value))value=[];value=value.filter(item=>item!==choice);if(el.checked)value.push(choice)}
 else if(kind==='truefalse'){value=typeof value==='object'?value:{};value[key]=el.value==='true'}
 else if(kind==='match'){value=typeof value==='object'?value:{};if(el.value)value[key]=el.value;else delete value[key]}
 else if(kind==='grid-single'){value=typeof value==='object'?value:{};value[key]=el.value}
 else if(kind==='grid-multi'){value=typeof value==='object'?value:{};const selected=Array.isArray(value[key])?value[key]:[];value[key]=el.checked?[...new Set([...selected,choice])]:selected.filter(item=>item!==choice)}
 saveConfiguredAnswer(id,value)}
function moveConfiguredOrder(id,from,to){const question=state.soal.find(item=>item.id===id);if(!question)return;const config=question.konfigurasi||{};const current=configuredAnswer(question);const order=Array.isArray(current)&&current.length?current:((config.choices||[]).map(item=>item.id));if(from<0||from>=order.length||to<0||to>=order.length)return;const next=[...order];const [item]=next.splice(from,1);next.splice(to,0,item);saveConfiguredAnswer(id,next)}
function renderConfiguredAnswer(question){const cfg=question.konfigurasi||{};const id=esc(question.id);const answer=configuredAnswer(question);const selected=answer===null?'':answer;const label=(text)=>'<span class="option-label">'+text+'</span>';const choices=cfg.choices||[];let html='<div class="configured-answer">';
 if(question.tipe==='pg_tunggal'||question.tipe==='pg_kompleks'){
  choices.forEach((item,index)=>{const checked=question.tipe==='pg_tunggal'?selected===item.id:(Array.isArray(selected)&&selected.includes(item.id));const inputType=question.tipe==='pg_tunggal'?'radio':'checkbox';html+='<label class="option '+(checked?'selected':'')+'"><input type="'+inputType+'" name="cfg_'+id+(question.tipe==='pg_kompleks'?'_'+esc(item.id):'')+'" data-config-answer="'+(question.tipe==='pg_kompleks'?'multi':'single')+'" data-id="'+id+'" data-value="'+esc(item.id)+'" '+(checked?'checked':'')+'><span class="option-label"><strong>'+String.fromCharCode(65+index)+'.</strong> '+esc(item.text)+'</span></label>'});
 }else if(question.tipe==='dropdown'){
  html+='<label class="select-answer-label" for="cfg_'+id+'">Pilih satu jawaban</label><select class="input" id="cfg_'+id+'" data-config-answer="select" data-id="'+id+'"><option value="">Pilih jawaban</option>'+choices.map(item=>'<option value="'+esc(item.id)+'" '+(selected===item.id?'selected':'')+'>'+esc(item.text)+'</option>').join('')+'</select>';
 }else if(question.tipe==='benar_salah'){
  html+='<div class="table-responsive"><table class="config-table"><thead><tr><th>Pernyataan</th><th>Benar</th><th>Salah</th></tr></thead><tbody>'+(cfg.statements||[]).map(row=>'<tr><th>'+esc(row.text)+'</th><td><input type="radio" name="cfg_'+id+'_'+esc(row.id)+'" value="true" data-config-answer="truefalse" data-id="'+id+'" data-key="'+esc(row.id)+'" '+(selected&&selected[row.id]===true?'checked':'')+'></td><td><input type="radio" name="cfg_'+id+'_'+esc(row.id)+'" value="false" data-config-answer="truefalse" data-id="'+id+'" data-key="'+esc(row.id)+'" '+(selected&&selected[row.id]===false?'checked':'')+'></td></tr>').join('')+'</tbody></table></div>';
 }else if(question.tipe==='menjodohkan'){
  html+='<div class="match-answer">'+(cfg.left||[]).map(row=>'<label class="match-row"><span>'+esc(row.text)+'</span><select class="input" data-config-answer="match" data-id="'+id+'" data-key="'+esc(row.id)+'"><option value="">Pilih pasangan</option>'+(cfg.right||[]).map(item=>'<option value="'+esc(item.id)+'" '+(selected&&selected[row.id]===item.id?'selected':'')+'>'+esc(item.text)+'</option>').join('')+'</select></label>').join('')+'</div>';
 }else if(question.tipe==='isian_singkat'||question.tipe==='tanggal'||question.tipe==='waktu'||question.tipe==='uraian'){
  const text=typeof selected==='string'?selected:'';const inputType=question.tipe==='tanggal'?'date':question.tipe==='waktu'?'time':'text';const maxLength=Number(cfg.textMaxLength||0);const maxAttr=maxLength>0?' maxlength="'+maxLength+'"':'';const minLength=Number(cfg.textMinLength||0);const ruleHint=cfg.validationMessage||((minLength&&maxLength)?'Jawaban '+minLength+'–'+maxLength+' karakter.':minLength?'Jawaban minimal '+minLength+' karakter.':maxLength?'Jawaban maksimal '+maxLength+' karakter.':'');const defaultHint=question.tipe==='uraian'?'Jawaban uraian akan dinilai guru menggunakan rubrik.':'Periksa kembali ejaan dan angka jawabanmu.';html+=question.tipe==='uraian'?'<textarea class="textarea" data-config-answer="text" data-id="'+id+'"'+maxAttr+' placeholder="Tulis jawaban Anda di sini...">'+esc(text)+'</textarea><p class="muted">'+esc(ruleHint||defaultHint)+'</p>':'<label class="select-answer-label" for="cfg_'+id+'">'+(question.tipe==='tanggal'?'Tanggal':question.tipe==='waktu'?'Waktu':'Jawaban singkat')+'</label><input class="input" id="cfg_'+id+'" type="'+inputType+'"'+(inputType==='text'?maxAttr:'')+' data-config-answer="text" data-id="'+id+'" value="'+esc(text)+'" placeholder="Ketik jawaban Anda..."><p class="muted">'+esc(inputType==='text'?(ruleHint||defaultHint):'Pilih jawaban menggunakan kontrol yang tersedia.')+'</p>';
 }else if(question.tipe==='skala_linear'||question.tipe==='rating'){
  const min=question.tipe==='rating'?1:Number(cfg.scaleMin||1);const max=question.tipe==='rating'?Number(cfg.ratingMax||5):Number(cfg.scaleMax||5);html+='<div class="scale-labels"><span>'+esc(cfg.scaleMinLabel||'')+'</span><span>'+esc(cfg.scaleMaxLabel||'')+'</span></div><div class="scale-options" role="radiogroup">';for(let value=min;value<=max&&value<min+11;value++){html+='<label class="option scale-option '+(selected===value?'selected':'')+'"><input type="radio" name="cfg_'+id+'" value="'+value+'" data-config-answer="number" data-id="'+id+'" '+(selected===value?'checked':'')+'><span>'+((question.tipe==='rating')?'★'.repeat(value):value)+'</span></label>'}html+='</div>';
 }else if(question.tipe==='kisi_pg'||question.tipe==='kisi_checkbox'){
  const multiple=question.tipe==='kisi_checkbox';html+='<div class="table-responsive"><table class="config-table"><thead><tr><th>Pernyataan</th>'+(cfg.columns||[]).map(col=>'<th>'+esc(col.text)+'</th>').join('')+'</tr></thead><tbody>'+(cfg.rows||[]).map(row=>'<tr><th>'+esc(row.text)+'</th>'+(cfg.columns||[]).map(col=>{const checked=multiple?Boolean(selected&&selected[row.id]&&selected[row.id].includes(col.id)):Boolean(selected&&selected[row.id]===col.id);return '<td><input type="'+(multiple?'checkbox':'radio')+'" name="cfg_'+id+'_'+esc(row.id)+'" value="'+esc(col.id)+'" data-config-answer="'+(multiple?'grid-multi':'grid-single')+'" data-id="'+id+'" data-key="'+esc(row.id)+'" data-value="'+esc(col.id)+'" '+(checked?'checked':'')+'></td>'}).join('')+'</tr>').join('')+'</tbody></table></div>';
 }else if(question.tipe==='susun_urutan'){
  const order=Array.isArray(selected)&&selected.length?selected:choices.map(item=>item.id);html+='<p class="muted">Seret untuk mengurutkan atau gunakan tombol panah.</p><div class="order-answer">'+order.map((choiceID,index)=>{const item=choices.find(value=>value.id===choiceID);if(!item)return '';return '<div class="option order-item" draggable="true" data-order-drag data-id="'+id+'" data-value="'+esc(choiceID)+'"><span class="drag-handle" aria-hidden="true">⠿</span><strong>'+(index+1)+'.</strong><span class="order-text">'+esc(item.text)+'</span><button class="btn btn-outline btn-sm" type="button" data-action="config-order-move" data-id="'+id+'" data-index="'+index+'" data-target="'+Math.max(0,index-1)+'" '+(index===0?'disabled':'')+' aria-label="Pindahkan ke atas">↑</button><button class="btn btn-outline btn-sm" type="button" data-action="config-order-move" data-id="'+id+'" data-index="'+index+'" data-target="'+Math.min(order.length-1,index+1)+'" '+(index===order.length-1?'disabled':'')+' aria-label="Pindahkan ke bawah">↓</button></div>'}).join('')+'</div>';
 }else if(question.tipe==='unggah_berkas'){
  const allowed=cfg.allowedFileTypes||['pdf','docx','xlsx','png','jpg','jpeg'];const maxFiles=Number(cfg.maxFiles||3);const maxMB=Number(cfg.maxFileSizeMB||10);const files=state.berkas[question.id]||[];const accept=allowed.map(ext=>'.'+ext).join(',');html+='<div class="file-answer"><label class="file-drop" for="cfg_file_'+id+'">＋ Pilih berkas jawaban<input class="sr-only" id="cfg_file_'+id+'" type="file" data-config-answer="upload" data-id="'+id+'" accept="'+esc(accept)+'" '+(files.length>=maxFiles?'disabled':'')+'></label><p class="muted">Format: '+esc(allowed.map(ext=>ext.toUpperCase()).join(', '))+' · Maks. '+maxFiles+' file · '+maxMB+' MB per file. Berkas hanya dapat dilihat olehmu dan guru berwenang.</p><ul class="file-list">'+files.map(file=>'<li><a href="'+API+'/ujian-online/'+encodeURIComponent(question.ujianId)+'/soal/'+encodeURIComponent(question.id)+'/file/'+encodeURIComponent(file.id)+'" target="_blank" rel="noopener">'+esc(file.namaFile)+' ('+(Number(file.ukuran||0)/1048576).toFixed(2)+' MB)</a><button class="btn btn-outline btn-sm" type="button" data-action="config-file-delete" data-id="'+id+'" data-file-id="'+esc(file.id)+'" aria-label="Hapus '+esc(file.namaFile)+'">Hapus</button></li>').join('')+'</ul><div class="file-status" id="file_status_'+id+'" role="status"></div></div>';
 }
 return html+'</div>';
}
async function uploadConfiguredFile(input){const file=input.files&&input.files[0];const id=input.dataset.id||'';input.value='';if(!file||!state.currentUjian)return;const question=state.soal.find(item=>item.id===id);if(!question)return;const status=document.getElementById('file_status_'+id);if(status)status.textContent='Mengunggah berkas…';const body=new FormData();body.append('file',file);const revisionKey=state.revisingResponses?stableRevisionKey('upload:'+id):'';const headers=revisionKey?{'Idempotency-Key':revisionKey}:{};try{const response=await fetch(API+'/ujian-online/'+encodeURIComponent(state.currentUjian.id)+'/soal/'+encodeURIComponent(id)+'/file',{method:'POST',body,headers,credentials:'include'});const data=await response.json();if(!response.ok)throw new Error(data.error||'Berkas gagal diunggah');if(revisionKey)delete state.revisionRequestKeys['upload:'+id];state.berkas[id]=[...(state.berkas[id]||[]),data];let ids=[];try{ids=JSON.parse(state.jawaban[id]||'[]')}catch(_){}if(!Array.isArray(ids))ids=[];if(!ids.includes(data.id))ids.push(data.id);state.jawaban[id]=JSON.stringify(ids);renderSoal();const updated=document.getElementById('file_status_'+id);if(updated)updated.textContent='Berkas berhasil diunggah dan tersimpan.'}catch(error){if(status)status.textContent=error.message||'Gagal mengunggah berkas.';renderSoal();const restored=document.getElementById('file_status_'+id);if(restored)restored.textContent=error.message||'Gagal mengunggah berkas.'}}
async function deleteConfiguredFile(id,fileId){if(!state.currentUjian)return;const key='delete:'+fileId;const revisionKey=state.revisingResponses?stableRevisionKey(key):'';try{const headers=revisionKey?{'Idempotency-Key':revisionKey}:{};const response=await fetch(API+'/ujian-online/'+encodeURIComponent(state.currentUjian.id)+'/soal/'+encodeURIComponent(id)+'/file/'+encodeURIComponent(fileId),{method:'DELETE',headers,credentials:'include'});if(!response.ok){const data=await response.json();throw new Error(data.error||'Berkas gagal dihapus')}if(revisionKey)delete state.revisionRequestKeys[key];state.berkas[id]=(state.berkas[id]||[]).filter(file=>file.id!==fileId);let ids=[];try{ids=JSON.parse(state.jawaban[id]||'[]')}catch(_){}state.jawaban[id]=JSON.stringify(Array.isArray(ids)?ids.filter(value=>value!==fileId):[]);renderSoal()}catch(error){const status=document.getElementById('file_status_'+id);if(status)status.textContent=error.message||'Gagal menghapus berkas.'}}

function renderStimulus(s){
 const c=document.getElementById('stimulusContainer');
 if(s.stimulus&&s.stimulus.length){c.innerHTML=s.stimulus.map(x=>{if(x.jenis==='media_link')return '<a class="option" href="'+esc(x.konten)+'" target="_blank" rel="noopener noreferrer">↗ Buka media pendukung di tab baru</a>';if(x.jenis==='table'){const rows=String(x.konten||'').split(/\r?\n/).filter(row=>row.trim()!=='').map(row=>row.split('\t'));return '<div class="table-responsive"><table class="config-table"><tbody>'+rows.map((row,index)=>'<tr>'+row.map(cell=>'<'+(index===0?'th':'td')+'>'+esc(cell)+'</'+(index===0?'th':'td')+'>').join('')+'</tr>').join('')+'</tbody></table></div>'}return '<p style="white-space:pre-wrap">'+esc(x.konten||'')+'</p>'}).join('');return}
 c.innerHTML='<div class="stimulus-empty"><div><div style="font-size:28px">▤</div><strong>Soal ini tidak menggunakan stimulus tambahan.</strong><span>Fokus pada pertanyaan dan pilihan jawaban di sebelah kanan.</span></div></div>';
}

function renderPalette(targetId){
 const c=document.getElementById(targetId);if(!c)return;
 c.innerHTML=state.soal.map((s,i)=>{const isAnswered=hasExamAnswer(s,state.jawaban[s.id]);const isFlagged=Boolean(state.flagged&&state.flagged[s.id]);return '<button type="button" data-action="palette-question" data-index="'+i+'" class="'+(i===state.currentIdx?'current ':'')+(isAnswered?'answered ':'')+(isFlagged?'flagged':'')+'" aria-label="Buka soal '+(i+1)+(isAnswered?', sudah dijawab':', belum dijawab')+(isFlagged?', ditandai untuk diperiksa':'')+'">'+(i+1)+'</button>'}).join('');
}

function setExamFont(size){
 const shell=document.getElementById('examCard');shell.classList.remove('exam-font-small','exam-font-medium','exam-font-large');shell.classList.add('exam-font-'+size);
 document.querySelectorAll('.font-controls .btn').forEach(b=>b.classList.remove('active'));
 const button=document.querySelector('[data-action="font-'+size+'"]');if(button)button.classList.add('active');
 try{localStorage.setItem('ujian-font-scale',size)}catch(_){ }
}

function toggleFlag(){
 const s=state.soal[state.currentIdx];if(!s)return;
 state.flagged=state.flagged||{};state.flagged[s.id]=!state.flagged[s.id];persistExamLocalState();renderSoal();
 const status=document.getElementById('examSaveStatus');status.textContent=state.flagged[s.id]?'⚑ Soal ditandai untuk diperiksa':'✓ Tanda soal dihapus';
}

function jawab(soalId,val){
state.jawaban[soalId]=String(val);
sendJawaban(soalId,String(val));
renderSoal();
}
function toggleCheckboxAnswer(soalId,value,checked){let selected=[];try{const parsed=JSON.parse(state.jawaban[soalId]||'[]');if(Array.isArray(parsed))selected=parsed}catch(_){}selected=selected.filter(index=>Number.isInteger(index));if(checked&&!selected.includes(value))selected.push(value);if(!checked)selected=selected.filter(index=>index!==value);selected.sort((a,b)=>a-b);const answer=JSON.stringify(selected);state.jawaban[soalId]=answer;sendJawaban(soalId,answer);renderSoal();const control=Array.from(document.querySelectorAll('[data-change-action="checkbox-answer"]')).find(input=>input.dataset.id===soalId&&Number(input.value)===value);if(control)control.focus()}
function jawabTeks(soalId,val){state.jawaban[soalId]=val;
sendJawaban(soalId,val);
}
function sendJawaban(soalId,val){
state.answerSequence++;
const pending={soalId,val,sequence:state.answerSequence,revisionKey:state.revisingResponses?newIdempotencyKey():''};const existing=state.offlineQueue.findIndex(item=>item.soalId===soalId);
if(existing>=0)state.offlineQueue[existing]=pending;else state.offlineQueue.push(pending);
persistExamLocalState();
const status=document.getElementById('examSaveStatus');if(!isOnline){status.textContent='Offline — jawaban tersimpan lokal'}else{status.textContent='Menyimpan…';clearTimeout(state.saveTimer);state.saveTimer=setTimeout(()=>{void flushAnswerQueue()},350)}
}
function newIdempotencyKey(){return window.crypto&&typeof window.crypto.randomUUID==='function'?window.crypto.randomUUID():'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,c=>{const r=Math.random()*16|0;return(c==='x'?r:(r&3|8)).toString(16)})}
function stableRevisionKey(name){if(!state.revisionRequestKeys[name])state.revisionRequestKeys[name]=newIdempotencyKey();return state.revisionRequestKeys[name]}
async function flushAnswerQueue(){
 if(state.syncPromise)return state.syncPromise;
 state.syncPromise=(async()=>{
  while(isOnline&&state.offlineQueue.length&&state.currentUjian){
  const pending=state.offlineQueue[0];const fd=new FormData();fd.append('ujianSoalId',pending.soalId);fd.append('jawaban',pending.val);
   const branchQuestion=Boolean(state.soal.find(question=>question.id===pending.soalId&&question.hasBranching));
   try{const headers=state.revisingResponses&&pending.revisionKey?{'Idempotency-Key':pending.revisionKey}:{};const response=await fetch(API+'/ujian-online/'+state.currentUjian.id+'/jawab',{method:'POST',body:fd,headers,credentials:'include'});if(!response.ok)throw new Error('Jawaban belum tersimpan');const saved=await response.json().catch(()=>({}));
    const index=state.offlineQueue.findIndex(item=>item.soalId===pending.soalId&&item.sequence===pending.sequence);if(index>=0)state.offlineQueue.splice(index,1);persistExamLocalState();
    if(branchQuestion&&state.currentUjian){const activeIDs=new Set(Array.isArray(saved.activeSoalIds)?saved.activeSoalIds:[]);state.offlineQueue=state.offlineQueue.filter(item=>activeIDs.has(item.soalId));Object.keys(state.jawaban).forEach(id=>{if(!activeIDs.has(id)){delete state.jawaban[id];delete state.flagged[id]}});persistExamLocalState();await loadSoal(state.currentUjian.id)}
   }catch(_){const status=document.getElementById('examSaveStatus');if(status)status.textContent='Belum tersimpan — periksa koneksi';throw new Error('Jawaban belum tersimpan. Periksa koneksi lalu coba lagi.')}
  }
  const status=document.getElementById('examSaveStatus');if(status)status.textContent=state.offlineQueue.length?'Belum tersimpan':'✓ Tersimpan';
 })().finally(()=>{state.syncPromise=null});
 return state.syncPromise;
}
function prevSoal(){if(state.currentIdx>0){state.currentIdx--;renderSoal()}}
function nextSoal(){if(state.currentIdx<state.soal.length-1){state.currentIdx++;renderSoal()}}

function startTimer(mulaiStr,sisaOverride){
if(state.timerInterval)clearInterval(state.timerInterval);
const durasi=state.currentUjian?.durasiMenit||60;
const grace=state.currentUjian?.gracePeriodMenit||5;
// On reconnect, use stored mulai; otherwise parse from server
let mulaiMs;
if(mulaiStr){mulaiMs=new Date(mulaiStr).getTime();state.mulai=mulaiMs}
else{mulaiMs=state.mulai}
if(!mulaiMs)return;
const batasNormal=mulaiMs+durasi*60*1000;
const batasGrace=batasNormal+grace*60*1000;
// sisaOverride: server returns negative = in grace period
let inGrace=sisaOverride!==undefined&&sisaOverride<0;
state.timerInterval=setInterval(()=>{
const now=Date.now();
if(!inGrace){
// Normal countdown
const sisa=Math.max(0,Math.floor((batasNormal-now)/1000));
const h=Math.floor(sisa/3600),m=Math.floor((sisa%3600)/60),ss=sisa%60;
const el=document.getElementById('timer');
el.textContent=String(h).padStart(2,'0')+':'+String(m).padStart(2,'0')+':'+String(ss).padStart(2,'0');
el.className='exam-timer'+(sisa<300?' danger':sisa<600?' warning':'');
if(sisa<=0){inGrace=true}// switch to grace mode
}else{
// Grace period countdown
const sisaG=Math.max(0,Math.floor((batasGrace-now)/1000));
const m=Math.floor((sisaG%3600)/60),ss=sisaG%60;
const el=document.getElementById('timer');
el.textContent='GRACE '+String(m).padStart(2,'0')+':'+String(ss).padStart(2,'0');
el.className='exam-timer danger';
if(sisaG<=0){clearInterval(state.timerInterval);selesaiUjian(true)}
}
},1000);
}

async function selesaiUjian(auto){
if(!state.currentUjian||state.submitting||state.revisingResponses)return;
if(!isOnline){showOfflinePopup();return}
state.submitting=true;const submitButton=document.getElementById('confirmFinishAction');if(submitButton){submitButton.disabled=true;submitButton.textContent='Menyinkronkan…'}
try{
 clearTimeout(state.saveTimer);await flushAnswerQueue();
 if(state.offlineQueue.length)throw new Error('Masih ada jawaban yang belum tersimpan. Coba lagi sebelum mengirim.');
const r=await fetch(API+'/ujian-online/'+state.currentUjian.id+'/selesai',{method:'POST',credentials:'include'});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
 state.bolehEditRespons=Boolean(d.bolehEditRespons);state.revisingResponses=false;const editButton=document.getElementById('editResponsesBtn');if(editButton)editButton.classList.toggle('hidden',!state.bolehEditRespons);const revisionExit=document.getElementById('exitResponseEditBtn');if(revisionExit)revisionExit.classList.add('hidden');
 document.getElementById('scoreValue').textContent=d.menungguPenilaian?'—':Math.round(d.skor||0);
document.getElementById('scoreDetail').innerHTML=d.menungguPenilaian?'Jawaban sudah terkirim dan tersimpan.<br>Status: <strong>Menunggu penilaian guru untuk '+d.uraianMenunggu+' jawaban uraian</strong>':'Benar: <strong>'+d.benar+'</strong> dari <strong>'+d.total+'</strong> soal<br>Status: <strong>Selesai</strong>';
const key=examStorageKey();if(key)sessionStorage.removeItem(key);clearInterval(state.timerInterval);closeExamDialog(document.getElementById('confirmFinishModal'),false);
 hide(document.getElementById('examCard'));show(document.getElementById('preExamHeader'));show(document.getElementById('resultCard'));
const resultScore=document.getElementById('scoreValue');if(resultScore){resultScore.tabIndex=-1;resultScore.focus()}
}catch(e){if(auto){const status=document.getElementById('examSaveStatus');if(status)status.textContent=e.message||'Pengiriman otomatis tertunda — sambungkan internet.';showOfflinePopup()}else alert(e.message)}finally{state.submitting=false;if(submitButton){submitButton.disabled=false;submitButton.textContent='Kirim jawaban'}}
}
async function editSubmittedResponses(){if(!state.currentUjian?.id||!state.bolehEditRespons)return;await mulaiUjian(state.currentUjian.id)}
async function exitSubmittedResponseEdit(){if(!state.currentUjian||!state.revisingResponses||state.submitting)return;if(!isOnline){showOfflinePopup();return}state.submitting=true;try{await flushAnswerQueue();const response=await fetch(API+'/ujian-online/'+state.currentUjian.id+'/selesai',{method:'POST',credentials:'include'});const data=await response.json();if(!response.ok)throw new Error(data.error||'Gagal memuat hasil');state.revisingResponses=false;state.bolehEditRespons=Boolean(data.bolehEditRespons);document.getElementById('scoreValue').textContent=data.menungguPenilaian?'—':Math.round(data.skor||0);document.getElementById('scoreDetail').innerHTML=data.menungguPenilaian?'Jawaban tersimpan. <strong>Menunggu penilaian guru untuk '+data.uraianMenunggu+' jawaban uraian</strong>':'Benar: <strong>'+data.benar+'</strong> dari <strong>'+data.total+'</strong> soal<br>Status: <strong>Selesai</strong>';document.getElementById('editResponsesBtn').classList.toggle('hidden',!state.bolehEditRespons);document.getElementById('exitResponseEditBtn').classList.add('hidden');hide(document.getElementById('examCard'));show(document.getElementById('preExamHeader'));show(document.getElementById('resultCard'));}catch(error){alert(error.message||'Perubahan belum tersimpan.')}finally{state.submitting=false}}
function openFinishConfirmation(){if(!state.soal.length||state.revisingResponses)return;const answeredCount=state.soal.filter(question=>hasExamAnswer(question,state.jawaban[question.id])).length;const flaggedCount=state.soal.filter(question=>state.flagged[question.id]).length;document.getElementById('finishAnswered').textContent=answeredCount;document.getElementById('finishEmpty').textContent=state.soal.length-answeredCount;document.getElementById('finishFlagged').textContent=flaggedCount;openExamDialog(document.getElementById('confirmFinishModal'))}

document.addEventListener('visibilitychange',()=>{
if(document.hidden&&state.currentUjian&&!document.getElementById('examCard').classList.contains('hidden')){
fetch(API+'/ujian-online/'+state.currentUjian.id+'/tab-switch',{method:'POST',credentials:'include'}).then(r=>r.json()).then(d=>{
  if(d.locked){
    if(state.timerInterval)clearInterval(state.timerInterval);
    document.getElementById('scoreValue').textContent=d.menungguPenilaian?'—':Math.round(d.skor||0);
    document.getElementById('scoreDetail').innerHTML=d.menungguPenilaian?'Ujian dikunci karena sering berpindah tab.<br>Jawaban uraian menunggu penilaian guru.':'Ujian dikunci (terlalu sering pindah tab).<br>Skor: <strong>'+Math.round(d.skor||0)+'</strong>';
    hide(document.getElementById('examCard'));show(document.getElementById('preExamHeader'));show(document.getElementById('resultCard'));
  }
}).catch(()=>{});
}
});

function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
</script>
</body>
</html>`
