package main

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) serveOrangTuaPortalPage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	siteKey := os.Getenv("TURNSTILE_SITE_KEY")
	html := strings.Replace(ortuPortalHTML, "{{TURNSTILE_SITE_KEY}}", siteKey, 1)
	return c.SendString(html)
}

var ortuPortalHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<meta name="theme-color" content="#ffffff">
<title>Portal Orang Tua — PKBM Tunas Ilmu</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit" async defer></script>
<style>
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
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:var(--background);color:var(--foreground);min-height:100vh;min-height:100dvh;-webkit-font-smoothing:antialiased;-webkit-tap-highlight-color:transparent;touch-action:manipulation}
.wrap{max-width:480px;margin:0 auto;padding:24px 16px;padding:24px max(16px,env(safe-area-inset-left))}

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

.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;white-space:nowrap;height:44px;padding:0 20px;border-radius:var(--radius);font-size:15px;font-weight:500;font-family:inherit;cursor:pointer;transition:background .15s,border-color .15s,color .15s;border:1px solid transparent;text-decoration:none;-webkit-appearance:none;appearance:none}
.btn:disabled{opacity:.5;pointer-events:none}
.btn:active{transform:scale(0.98)}
.btn-primary{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}
.btn-primary:hover{background:#27272a}
.btn-outline{background:var(--background);color:var(--foreground);border-color:var(--input)}
.btn-outline:hover{background:var(--accent);color:var(--accent-foreground)}
.btn-ghost{background:transparent;color:var(--muted-foreground);border-color:transparent}
.btn-ghost:hover{background:var(--accent);color:var(--foreground)}
.btn-sm{height:36px;padding:0 14px;font-size:13px}
.btn-lg{height:48px;padding:0 24px;font-size:16px}

.error-box{background:#fef2f2;border:1px solid #fecaca;color:var(--destructive);padding:12px 14px;border-radius:var(--radius);font-size:14px;margin-bottom:16px;display:none}
.error-box.show{display:block}

.child-card{border:1px solid var(--border);border-radius:var(--radius);padding:16px;margin-bottom:8px;cursor:pointer;transition:all .15s}
.child-card:hover{border-color:#a1a1aa;box-shadow:0 1px 3px 0 rgb(0 0 0 / 0.1)}
.child-card:active{transform:scale(0.99)}
.child-card h3{font-size:15px;font-weight:600;margin:0 0 4px}
.child-card .meta{font-size:13px;color:var(--muted-foreground);display:flex;flex-wrap:wrap;gap:4px 12px;margin:0}

.tabs{display:flex;gap:6px;margin-bottom:16px;overflow-x:auto;-webkit-overflow-scrolling:touch;scrollbar-width:none;padding-bottom:2px}
.tabs::-webkit-scrollbar{display:none}
.tab{padding:8px 16px;border-radius:var(--radius);font-size:13px;font-weight:500;cursor:pointer;border:1px solid var(--border);background:var(--background);color:var(--foreground);transition:all .15s;white-space:nowrap;flex-shrink:0}
.tab:hover{background:var(--accent)}
.tab.active{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}

.table-wrap{overflow-x:auto;-webkit-overflow-scrolling:touch;margin:0 -24px;padding:0 24px}
.table{width:100%;border-collapse:collapse;font-size:13px;min-width:400px}
.table th,.table td{padding:10px 12px;text-align:left;border-bottom:1px solid var(--border)}
.table th{font-weight:500;color:var(--muted-foreground);font-size:12px;text-transform:uppercase;letter-spacing:0.05em;background:var(--secondary);position:sticky;top:0}
.table td{font-size:14px}
.table tbody tr:hover{background:var(--secondary)}

.badge{display:inline-flex;align-items:center;border-radius:9999px;padding:2px 10px;font-size:12px;font-weight:500;line-height:1.4}
.badge-success{background:#dcfce7;color:#166534}
.badge-destructive{background:#fef2f2;color:#991b1b}
.badge-warning{background:#fefce8;color:#854d0e}
.badge-secondary{background:var(--secondary);color:var(--secondary-foreground);border:1px solid var(--border)}

.empty-state{text-align:center;padding:32px 16px;color:var(--muted-foreground)}
.empty-state p{font-size:14px}

.section-title{font-size:14px;font-weight:600;margin:16px 0 8px;color:var(--foreground)}
.callout{background:var(--secondary);border:1px solid var(--border);border-radius:var(--radius);padding:14px;font-size:14px;line-height:1.7;color:var(--foreground)}
.decision{font-weight:600;font-size:15px;margin-top:12px}
.decision-sub{font-size:13px;color:var(--muted-foreground);margin-top:2px}

.hidden{display:none!important}

@media(max-width:640px){
  .wrap{padding:12px 12px;padding:12px max(12px,env(safe-area-inset-left))}
  .card-header{padding:20px 16px 0}
  .card-content{padding:20px 16px}
  .card-footer{padding:0 16px 20px}
  h1{font-size:16px}
  .table-wrap{margin:0 -16px;padding:0 16px}
  .tabs{gap:4px}
  .tab{padding:7px 12px;font-size:12px}
}
@media(max-width:360px){
  .wrap{padding:8px 8px;padding:8px max(8px,env(safe-area-inset-left))}
  .card-header{padding:16px 12px 0}
  .card-content{padding:16px 12px}
  .card-footer{padding:0 12px 16px}
  .input{height:40px;font-size:14px}
  .btn{height:40px;font-size:14px;padding:0 16px}
  .btn-lg{height:44px}
}
</style>
</head>
<body>
<div class="wrap">

<!-- Login -->
<div id="loginCard" class="card">
  <div class="card-header"><h1>Portal Orang Tua</h1><p class="desc">Masukkan NISN anak dan tanggal lahir untuk melihat data akademik.</p></div>
  <div class="card-content">
    <div id="loginError" class="error-box"></div>
    <div class="form-group">
      <label for="nisn">NISN Anak</label>
      <input class="input" type="text" id="nisn" placeholder="10 digit NISN" maxlength="10" inputmode="numeric" autocomplete="off">
    </div>
    <div class="form-group">
      <label for="tanggalLahir">Tanggal Lahir Anak</label>
      <input class="input" type="text" id="tanggalLahir" placeholder="DDMMYYYY (contoh: 15082010)" maxlength="8" inputmode="numeric" autocomplete="off">
    </div>
    <div class="form-group" id="turnstileContainer">
      <div class="cf-turnstile" data-sitekey="{{TURNSTILE_SITE_KEY}}" data-theme="light" data-callback="onTurnstileSuccess"></div>
    </div>
  </div>
  <div class="card-footer">
    <button class="btn btn-primary btn-lg" onclick="doLogin()" id="loginBtn" style="width:100%">Masuk</button>
  </div>
</div>

<!-- Portal -->
<div id="portalCard" class="card hidden">
  <div class="card-header">
    <div style="display:flex;align-items:center;justify-content:space-between">
      <h1>Data Anak</h1>
      <button class="btn btn-ghost btn-sm" onclick="doLogout()">Keluar</button>
    </div>
  </div>
  <div class="card-content">
    <div id="childrenList"></div>
  </div>
</div>

<!-- Child Detail -->
<div id="detailCard" class="card hidden">
  <div class="card-header">
    <div style="display:flex;align-items:center;justify-content:space-between">
      <h1 id="detailName">Detail Anak</h1>
      <button class="btn btn-ghost btn-sm" onclick="showPortal()">&larr; Kembali</button>
    </div>
  </div>
  <div class="card-content">
    <div class="tabs">
      <div class="tab active" onclick="showTab('nilai',this)">Nilai</div>
      <div class="tab" onclick="showTab('presensi',this)">Presensi</div>
      <div class="tab" onclick="showTab('rapor',this)">Rapor</div>
    </div>
    <div id="tabContent"></div>
  </div>
</div>

</div>

<script>
const API='/api';
let state={token:'',anakId:'',anakList:[]};

function show(el){el.classList.remove('hidden')}
function hide(el){el.classList.add('hidden')}
function showError(id,msg){const e=document.getElementById(id);e.textContent=msg;show(e)}
function showPortal(){show(document.getElementById('portalCard'));hide(document.getElementById('detailCard'));hide(document.getElementById('loginCard'))}
function showLogin(){show(document.getElementById('loginCard'));hide(document.getElementById('portalCard'));hide(document.getElementById('detailCard'));state.token='';state.anakId=''}

async function doLogin(){
const nisn=document.getElementById('nisn').value.trim();
const tanggalLahir=document.getElementById('tanggalLahir').value;
if(!nisn||!tanggalLahir){showError('loginError','NISN dan tanggal lahir wajib diisi.');return}
document.getElementById('loginError').classList.remove('show');
document.getElementById('loginBtn').disabled=true;document.getElementById('loginBtn').textContent='Masuk...';
try{
const tw=document.querySelector('[name="cf-turnstile-response"]');
const token=tw?tw.value:'';
const r=await fetch(API+'/orang-tua/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({nisn,tanggalLahir,'cf-turnstile-response':token})});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
state.token=d.accessToken;
await loadAnak();
hide(document.getElementById('loginCard'));show(document.getElementById('portalCard'));
}catch(e){showError('loginError',e.message)}
finally{document.getElementById('loginBtn').disabled=false;document.getElementById('loginBtn').textContent='Masuk'}
}

async function loadAnak(){
const r=await fetch(API+'/orang-tua/anak',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
state.anakList=Array.isArray(d)?d:[];
const c=document.getElementById('childrenList');
if(!state.anakList.length){c.innerHTML='<div class="empty-state"><p>Tidak ada data anak ditemukan.</p></div>';return}
c.innerHTML=state.anakList.map(a=>{
const kelas='Kelas '+String(a.kelas?.jenjang||'')+esc(a.kelas?.namaRombel||'');
return '<div class="child-card" onclick="openAnak(\''+a.id+'\')"><h3>'+esc(a.nama)+'</h3><div class="meta"><span>NISN: '+esc(a.nisn)+'</span><span>'+kelas+'</span></div></div>'
}).join('');
}

async function openAnak(id){
state.anakId=id;
const anak=state.anakList.find(a=>a.id===id);
document.getElementById('detailName').textContent=anak?anak.nama:'Detail Anak';
hide(document.getElementById('portalCard'));show(document.getElementById('detailCard'));
showTab('nilai',document.querySelector('.tab'));
}

async function showTab(tab,el){
document.querySelectorAll('.tab').forEach(n=>n.classList.remove('active'));
if(el)el.classList.add('active');
const c=document.getElementById('tabContent');
c.innerHTML='<div class="empty-state"><p>Memuat...</p></div>';
try{
if(tab==='nilai'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/nilai',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
if(!d.length){c.innerHTML='<div class="empty-state"><p>Belum ada data nilai.</p></div>';return}
c.innerHTML='<div class="table-wrap"><table class="table"><thead><tr><th>Mapel</th><th>NP</th><th>NK</th><th>Prd NP</th><th>Prd NK</th></tr></thead><tbody>'+d.map(r=>'<tr><td>'+esc(r.mapel?.namaMapel||'-')+'</td><td>'+(r.npAkhir!=null?Number(r.npAkhir).toFixed(1):'-')+'</td><td>'+(r.nkAkhir!=null?Number(r.nkAkhir).toFixed(1):'-')+'</td><td>'+(r.predikatNP||'-')+'</td><td>'+(r.predikatNK||'-')+'</td></tr>').join('')+'</tbody></table></div>';
}else if(tab==='presensi'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/presensi',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
if(!d.length){c.innerHTML='<div class="empty-state"><p>Belum ada data presensi.</p></div>';return}
c.innerHTML='<div class="table-wrap"><table class="table"><thead><tr><th>Tanggal</th><th>Status</th><th>Catatan</th></tr></thead><tbody>'+d.map(r=>{const cls=r.statusKehadiran==='Hadir'?'badge-success':r.statusKehadiran==='Alfa'?'badge-destructive':'badge-warning';return'<tr><td>'+String(r.presensi?.tanggal||'').slice(0,10)+'</td><td><span class="badge '+cls+'">'+esc(r.statusKehadiran)+'</span></td><td>'+esc(r.catatan||'-')+'</td></tr>'}).join('')+'</tbody></table></div>';
}else if(tab==='rapor'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/rapor',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
let html='';
if(d.rekapNilai&&d.rekapNilai.length){
html+='<div class="section-title">Rekap Nilai</div>';
html+='<div class="table-wrap"><table class="table"><thead><tr><th>Mapel</th><th>NP</th><th>NK</th><th>NA</th></tr></thead><tbody>'+d.rekapNilai.map(r=>'<tr><td>'+esc(r.mapel?.namaMapel||'-')+'</td><td>'+(r.npAkhir!=null?Number(r.npAkhir).toFixed(1):'-')+'</td><td>'+(r.nkAkhir!=null?Number(r.nkAkhir).toFixed(1):'-')+'</td><td>'+(r.naAkhir!=null?Number(r.naAkhir).toFixed(1):'-')+'</td></tr>').join('')+'</tbody></table></div>';
}else{html+='<div class="empty-state"><p>Belum ada data rekap.</p></div>'}
if(d.catatanRapor&&d.catatanRapor.catatanWali){
html+='<div class="section-title">Catatan Wali Kelas</div>';
html+='<div class="callout">'+esc(d.catatanRapor.catatanWali)+'</div>';
if(d.catatanRapor.naikKelas!=null){
html+='<div class="decision">'+(d.catatanRapor.naikKelas?'Naik Kelas':'Tidak Naik Kelas')+'</div>';
if(d.catatanRapor.kenaikanKe)html+='<div class="decision-sub">Kenaikan ke: '+esc(d.catatanRapor.kenaikanKe)+'</div>';
}
}
c.innerHTML=html;
}
}catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

function doLogout(){state.token='';showLogin()}
function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
</script>
</body>
</html>`
