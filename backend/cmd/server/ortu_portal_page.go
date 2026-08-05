package main

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) serveOrangTuaPortalPage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	siteKey := os.Getenv("TURNSTILE_SITE_KEY")
	if siteKey == "" {
		siteKey = "1x00000000000000000000AA"
	}
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
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
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
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:#f4f6fb;color:var(--foreground);min-height:100vh;min-height:100dvh;-webkit-font-smoothing:antialiased;-webkit-tap-highlight-color:transparent}
.wrap{max-width:1120px;margin:0 auto;padding:32px 24px}
.login-card{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);min-height:620px;border-radius:18px;border-color:#dfe3eb;box-shadow:0 18px 50px rgba(15,23,42,.10)}
.login-brand{position:relative;display:flex;flex-direction:column;justify-content:space-between;overflow:hidden;padding:52px 48px;background:linear-gradient(135deg,#536dff 0%,#3441ed 56%,#3d42d9 100%);color:#fff}
.login-brand:before,.login-brand:after{content:"";position:absolute;border-radius:999px;background:rgba(255,255,255,.10);filter:blur(2px);pointer-events:none}
.login-brand:before{width:300px;height:300px;left:-160px;top:-130px}.login-brand:after{width:320px;height:320px;right:-170px;bottom:-190px}
.brand-logo,.brand-copy,.brand-footer{position:relative;z-index:1}.brand-logo{display:flex;align-items:center;gap:14px}.brand-mark{display:flex;width:54px;height:54px;align-items:center;justify-content:center;border-radius:18px;background:#fff;color:#4354f5;font-size:24px;font-weight:800;box-shadow:0 8px 18px rgba(15,23,42,.15)}
.brand-name{font-size:22px;font-weight:800;letter-spacing:-.04em}.brand-subtitle{margin-top:3px;font-size:13px;color:rgba(255,255,255,.80);font-weight:600}.brand-copy{margin:auto 0;padding:60px 0 42px}.brand-copy h2{max-width:430px;color:#fff;font-size:42px;line-height:1.12;letter-spacing:-.05em;font-weight:800}.brand-copy p{max-width:470px;margin-top:22px;color:rgba(255,255,255,.82);font-size:16px;line-height:1.7}.brand-protected{display:flex;align-items:center;gap:14px;margin-top:34px}.brand-protected-icon{display:flex;width:44px;height:44px;align-items:center;justify-content:center;border-radius:14px;background:rgba(255,255,255,.15)}.brand-protected strong{display:block;font-size:14px}.brand-protected span{display:block;margin-top:4px;color:rgba(255,255,255,.72);font-size:12px}.brand-footer{color:rgba(255,255,255,.65);font-size:12px;font-weight:600}
.login-form-panel{display:flex;flex-direction:column;justify-content:center;padding:52px 56px;background:#fff}.login-heading{margin-bottom:28px}.login-heading .eyebrow{color:#4354f5;font-size:13px;font-weight:800;letter-spacing:.04em;text-transform:uppercase}.login-heading h2{margin-top:10px;color:#101828;font-size:32px;line-height:1.15;letter-spacing:-.045em;font-weight:800}.login-heading p{margin-top:10px;color:#64748b;font-size:14px;line-height:1.6}.login-content{padding:0}.login-footer{padding:0;margin-top:22px}.login-footer .btn-lg{height:50px}
.turnstile-wrap{min-height:70px;margin:4px 0 10px;display:flex;justify-content:center;align-items:center}
.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);box-shadow:0 1px 2px 0 rgb(0 0 0 / 0.05);overflow:hidden;margin-bottom:12px}
.card-header{padding:20px 20px 0}
.card-content{padding:20px}
.card-footer{padding:0 20px 20px;display:flex;gap:8px}
h1{font-size:18px;font-weight:600;letter-spacing:-0.025em;margin:0}
p.desc{font-size:13px;color:var(--muted-foreground);margin-top:2px}
.form-group{margin-bottom:14px}
label{display:block;font-size:13px;font-weight:500;margin-bottom:5px}
.input{width:100%;height:42px;padding:0 12px;border:1px solid var(--input);border-radius:var(--radius);font-size:15px;font-family:inherit;background:var(--background);color:var(--foreground)}
.input:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;height:42px;padding:0 18px;border-radius:var(--radius);font-size:14px;font-weight:500;font-family:inherit;cursor:pointer;border:1px solid transparent}
.btn:disabled{opacity:.5;pointer-events:none}
.btn-primary{background:var(--primary);color:var(--primary-foreground)}
.btn-ghost{background:transparent;color:var(--muted-foreground);border-color:transparent}
.btn-sm{height:34px;padding:0 12px;font-size:12px}
.btn-lg{height:46px;width:100%;font-size:15px}
.error-box{background:#fef2f2;border:1px solid #fecaca;color:var(--destructive);padding:10px 12px;border-radius:var(--radius);font-size:13px;margin-bottom:14px;display:none}
.error-box.show{display:block}

.top-bar{display:flex;align-items:center;justify-content:space-between;margin-bottom:12px}
.top-bar h2{font-size:16px;font-weight:600}
.child-select{display:flex;gap:6px;margin-bottom:12px;overflow-x:auto;-webkit-overflow-scrolling:touch;scrollbar-width:none}
.child-select::-webkit-scrollbar{display:none}
.child-btn{padding:6px 14px;border-radius:var(--radius);font-size:12px;font-weight:500;border:1px solid var(--border);background:var(--background);color:var(--foreground);white-space:nowrap;cursor:pointer;flex-shrink:0}
.child-btn.active{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}

.tabs{display:flex;gap:4px;margin-bottom:14px;overflow-x:auto;-webkit-overflow-scrolling:touch;scrollbar-width:none;padding-bottom:2px}
.tabs::-webkit-scrollbar{display:none}
.tab{padding:7px 12px;border-radius:var(--radius);font-size:12px;font-weight:500;cursor:pointer;border:1px solid var(--border);background:var(--background);color:var(--foreground);white-space:nowrap;flex-shrink:0}
.tab.active{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}

.table-wrap{overflow-x:auto;-webkit-overflow-scrolling:touch}
.table{width:100%;border-collapse:collapse;font-size:13px}
.table th,.table td{padding:8px 10px;text-align:left;border-bottom:1px solid var(--border)}
.table th{font-weight:500;color:var(--muted-foreground);font-size:11px;text-transform:uppercase;letter-spacing:0.05em;background:var(--secondary)}
.table td{font-size:13px}

.badge{display:inline-flex;border-radius:9999px;padding:2px 8px;font-size:11px;font-weight:500;line-height:1.4}
.badge-success{background:#dcfce7;color:#166534}
.badge-destructive{background:#fef2f2;color:#991b1b}
.badge-warning{background:#fefce8;color:#854d0e}
.badge-secondary{background:var(--secondary);color:var(--secondary-foreground);border:1px solid var(--border)}

.profile-row{display:flex;justify-content:space-between;padding:8px 0;border-bottom:1px solid var(--border);font-size:13px}
.profile-row:last-child{border-bottom:none}
.profile-label{color:var(--muted-foreground)}
.profile-val{font-weight:500;text-align:right}

.chart-box{position:relative;height:200px;margin:12px 0}
.empty-state{text-align:center;padding:28px 12px;color:var(--muted-foreground);font-size:13px}

.chat-messages{max-height:400px;overflow-y:auto;display:flex;flex-direction:column;gap:8px;padding:8px 0}
.chat-msg{max-width:80%;padding:10px 14px;border-radius:12px;font-size:13px;line-height:1.5;word-break:break-word}
.chat-msg.sent{align-self:flex-end;background:var(--primary);color:var(--primary-foreground);border-bottom-right-radius:4px}
.chat-msg.received{align-self:flex-start;background:var(--secondary);border-bottom-left-radius:4px}
.chat-input-wrap{display:flex;gap:6px;margin-top:8px}
.chat-input-wrap .input{flex:1;height:38px;font-size:14px}
.chat-input-wrap .btn{height:38px;padding:0 14px}

.notif-item{padding:10px 0;border-bottom:1px solid var(--border);font-size:13px}
.notif-item:last-child{border-bottom:none}
.notif-title{font-weight:600;margin-bottom:2px}
.notif-time{font-size:11px;color:var(--muted-foreground)}
.notif-dot{width:8px;height:8px;border-radius:50%;background:var(--primary);display:inline-block;margin-right:6px}

.hidden{display:none!important}
@media(max-width:820px){.wrap{padding:18px 14px}.login-card{grid-template-columns:1fr;min-height:0}.login-brand{min-height:270px;padding:30px 28px}.brand-copy{padding:34px 0 20px}.brand-copy h2{font-size:30px}.brand-copy p{font-size:14px;margin-top:14px}.brand-protected{margin-top:20px}.brand-footer{display:none}.login-form-panel{padding:34px 28px 38px}.login-heading h2{font-size:27px}}
@media(max-width:480px){
  .wrap{padding:10px}
  .card-header{padding:16px 16px 0}
  .card-content{padding:16px}
  .card-footer{padding:0 16px 16px}
  .login-brand{padding:24px 20px;min-height:238px}.brand-mark{width:46px;height:46px;border-radius:15px;font-size:20px}.brand-name{font-size:18px}.brand-subtitle{font-size:11px}.brand-copy{padding:28px 0 12px}.brand-copy h2{font-size:26px}.brand-copy p{font-size:13px;line-height:1.5}.brand-protected{margin-top:16px}.login-form-panel{padding:28px 20px 30px}.login-heading{margin-bottom:22px}.login-heading h2{font-size:25px}
}
</style>
</head>
<body>
<div class="wrap">

<!-- Login -->
<div id="loginCard" class="login-card card">
  <div class="login-brand">
    <div class="brand-logo"><div class="brand-mark">TI</div><div><div class="brand-name">Tunas Ilmu Learn</div><div class="brand-subtitle">PKBM Tunas Ilmu</div></div></div>
    <div class="brand-copy"><h2>Portal Orang Tua yang Terhubung & Modern.</h2><p>Pantau perkembangan belajar, nilai, presensi, dan aktivitas peserta didik melalui satu akses yang aman.</p><div class="brand-protected"><div class="brand-protected-icon"><svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 3 5 6v5c0 4.5 2.9 8.5 7 10 4.1-1.5 7-5.5 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg></div><div><strong>Akses Terlindungi</strong><span>Verifikasi keamanan untuk setiap sesi masuk.</span></div></div></div>
    <div class="brand-footer">© 2026 PKBM Tunas Ilmu • Tunas Ilmu Learn</div>
  </div>
  <div class="login-form-panel">
    <div class="login-heading"><span class="eyebrow">Portal Orang Tua</span><h2>Masuk ke Portal Orang Tua</h2><p>Masukkan data anak untuk mengakses informasi pembelajaran.</p></div>
  <div class="card-content login-content">
    <div id="loginError" class="error-box"></div>
    <div class="form-group">
      <label>NISN Anak</label>
      <input class="input" type="text" id="nisn" placeholder="10 digit NISN" maxlength="10" inputmode="numeric">
    </div>
    <div class="form-group">
      <label>Tanggal Lahir Anak</label>
      <input class="input" type="text" id="tanggalLahir" placeholder="DDMMYYYY" maxlength="8" inputmode="numeric">
    </div>
    <div class="form-group turnstile-wrap"><div class="cf-turnstile" data-sitekey="{{TURNSTILE_SITE_KEY}}" data-theme="light" data-callback="onTurnstileSuccess" data-expired-callback="onTurnstileExpired" data-error-callback="onTurnstileError"></div></div>
  </div>
  <div class="card-footer login-footer"><button class="btn btn-primary btn-lg" onclick="doLogin()" id="loginBtn">Masuk</button></div>
  </div>
</div>

<!-- Portal Main -->
<div id="portalCard" class="hidden">
  <div class="top-bar">
    <h2>Portal Orang Tua</h2>
    <button class="btn btn-ghost btn-sm" onclick="doLogout()">Keluar</button>
  </div>
  <div class="child-select" id="childSelect"></div>
  <div class="tabs" id="mainTabs"></div>
  <div id="tabContent"></div>
</div>

</div>

<script>
const API='/api';
let state={token:'',anakId:'',anakList:[],anakData:null};
let turnstileToken='';
const TABS=[
  {id:'identitas',label:'Identitas',icon:'👤'},
  {id:'performa',label:'Performa',icon:'📊'},
  {id:'perilaku',label:'Perilaku',icon:'📝'},
  {id:'ujian',label:'Ujian',icon:'📝'},
  {id:'tugas',label:'Tugas',icon:'📋'},
  {id:'materi',label:'Materi',icon:'📚'},
  {id:'kalender',label:'Kalender',icon:'📅'},
  {id:'notif',label:'Notifikasi',icon:'🔔'},
  {id:'chat',label:'Chat',icon:'💬'},
  {id:'buku',label:'Buku',icon:'📖'}
];

function show(el){el.classList.remove('hidden')}
function hide(el){el.classList.add('hidden')}
function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
function hdr(){return{Authorization:'Bearer '+state.token}}
function onTurnstileSuccess(token){turnstileToken=token}
function onTurnstileExpired(){turnstileToken=''}
function onTurnstileError(){turnstileToken=''}
function resetTurnstile(){turnstileToken='';if(window.turnstile)window.turnstile.reset()}

async function doLogin(){
  const nisn=document.getElementById('nisn').value.trim();
  const tl=document.getElementById('tanggalLahir').value;
  if(!nisn||!tl){showErr('loginError','NISN dan tanggal lahir wajib diisi.');return}
  if(!turnstileToken){showErr('loginError','Silakan selesaikan verifikasi keamanan terlebih dahulu.');return}
  hideErr('loginError');
  const btn=document.getElementById('loginBtn');btn.disabled=true;btn.textContent='Masuk...';
  try{
    const r=await fetch(API+'/orang-tua/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({nisn,tanggalLahir:tl,'cf-turnstile-response':turnstileToken})});
    const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
    state.token=d.accessToken;
    await loadAnak();
    hide(document.getElementById('loginCard'));show(document.getElementById('portalCard'));
  }catch(e){showErr('loginError',e.message);resetTurnstile()}
  finally{btn.disabled=false;btn.textContent='Masuk'}
}

async function loadAnak(){
  const r=await fetch(API+'/orang-tua/anak',{headers:hdr()});
  const d=await r.json();state.anakList=Array.isArray(d)?d:[];
  renderChildSelect();
  if(state.anakList.length)selectAnak(state.anakList[0].id);
}

function renderChildSelect(){
  const el=document.getElementById('childSelect');
  el.innerHTML=state.anakList.map(a=>'<button class="child-btn'+(a.id===state.anakId?' active':'')+'" onclick="selectAnak(\''+a.id+'\')">'+esc(a.nama)+'</button>').join('');
}

async function selectAnak(id){
  state.anakId=id;state.anakData=state.anakList.find(a=>a.id===id);
  renderChildSelect();
  renderTabs();
  showTab('identitas');
}

function renderTabs(){
  document.getElementById('mainTabs').innerHTML=TABS.map((t,i)=>'<div class="tab'+(i===0?' active':'')+'" onclick="showTab(\''+t.id+'\',this)">'+t.icon+' '+t.label+'</div>').join('');
}

function showTab(tab,el){
  document.querySelectorAll('.tab').forEach(n=>n.classList.remove('active'));
  if(el)el.classList.add('active');
  const c=document.getElementById('tabContent');
  c.innerHTML='<div class="empty-state">Memuat...</div>';
  const m={
    identitas:loadIdentitas,performa:loadPerforma,perilaku:loadPerilaku,ujian:loadUjian,tugas:loadTugas,
    materi:loadMateri,kalender:loadKalender,notif:loadNotif,chat:loadChat,buku:loadBuku
  };
  if(m[tab])m[tab](c);
}

async function loadIdentitas(c){
  const a=state.anakData;if(!a)return;
  const kelas='Kelas '+String(a.kelas?.jenjang||'')+esc(a.kelas?.namaRombel||'');
  const pokjar=esc(a.kelas?.pokjar?.namaPokjar||'-');
  c.innerHTML='<div class="card"><div class="card-content">'+
    '<div style="text-align:center;margin-bottom:16px"><div style="width:64px;height:64px;border-radius:50%;background:var(--secondary);display:flex;align-items:center;justify-content:center;margin:0 auto 8px;font-size:24px">👤</div>'+
    '<div style="font-size:16px;font-weight:600">'+esc(a.nama)+'</div>'+
    '<div style="font-size:13px;color:var(--muted-foreground)">'+kelas+'</div></div>'+
    '<div class="profile-row"><span class="profile-label">NISN</span><span class="profile-val">'+esc(a.nisn)+'</span></div>'+
    '<div class="profile-row"><span class="profile-label">NIS</span><span class="profile-val">'+esc(a.nis)+'</span></div>'+
    '<div class="profile-row"><span class="profile-label">Jenis Kelamin</span><span class="profile-val">'+esc(a.jenisKelamin==='L'?'Laki-laki':'Perempuan')+'</span></div>'+
    '<div class="profile-row"><span class="profile-label">Pokjar</span><span class="profile-val">'+pokjar+'</span></div>'+
    '<div class="profile-row"><span class="profile-label">Status</span><span class="profile-val"><span class="badge badge-success">'+esc(a.status)+'</span></span></div>'+
    '</div></div>';
}

async function loadPerforma(c){
  const id=state.anakId;
  try{
    const [rNilai,rPresensi]=await Promise.all([
      fetch(API+'/orang-tua/anak/'+id+'/nilai',{headers:hdr()}).then(r=>r.json()),
      fetch(API+'/orang-tua/anak/'+id+'/presensi',{headers:hdr()}).then(r=>r.json())
    ]);
    let html='<div class="card"><div class="card-header"><h1 style="font-size:15px">Grafik Nilai</h1></div><div class="card-content"><div class="chart-box"><canvas id="chartNilai"></canvas></div></div></div>';
    html+='<div class="card"><div class="card-header"><h1 style="font-size:15px">Grafik Presensi</h1></div><div class="card-content"><div class="chart-box"><canvas id="chartPresensi"></canvas></div></div></div>';
    html+='<div class="card"><div class="card-header"><h1 style="font-size:15px">Rekap Nilai</h1></div><div class="card-content">';
    if(Array.isArray(rNilai)&&rNilai.length){
      html+='<div class="table-wrap"><table class="table"><thead><tr><th>Mapel</th><th>NP</th><th>NK</th><th>Predikat</th></tr></thead><tbody>';
      html+=rNilai.map(r=>'<tr><td>'+esc(r.mapel?.namaMapel||'-')+'</td><td>'+(r.npAkhir!=null?Number(r.npAkhir).toFixed(1):'-')+'</td><td>'+(r.nkAkhir!=null?Number(r.nkAkhir).toFixed(1):'-')+'</td><td>'+(r.predikatNP||'-')+'</td></tr>').join('');
      html+='</tbody></table></div>';
    }else html+='<div class="empty-state">Belum ada data nilai</div>';
    html+='</div></div>';
    c.innerHTML=html;
    // Draw charts
    if(Array.isArray(rNilai)&&rNilai.length){
      const labels=rNilai.map(r=>r.mapel?.namaMapel||'?');
      new Chart(document.getElementById('chartNilai'),{type:'bar',data:{labels,datasets:[
        {label:'NP',data:rNilai.map(r=>r.npAkhir),backgroundColor:'#3b82f6'},
        {label:'NK',data:rNilai.map(r=>r.nkAkhir),backgroundColor:'#22c55e'}
      ]},options:{responsive:true,maintainAspectRatio:false,plugins:{legend:{position:'bottom',labels:{boxWidth:12,font:{size:11}}}},scales:{y:{beginAtZero:true,max:100,ticks:{font:{size:10}}},x:{ticks:{font:{size:10}}}}}});
    }
    if(Array.isArray(rPresensi)&&rPresensi.length){
      const counts={Hadir:0,Sakit:0,Izin:0,Alfa:0};
      rPresensi.forEach(p=>{counts[p.statusKehadiran]=(counts[p.statusKehadiran]||0)+1});
      new Chart(document.getElementById('chartPresensi'),{type:'doughnut',data:{labels:Object.keys(counts),datasets:[{data:Object.values(counts),backgroundColor:['#22c55e','#f59e0b','#3b82f6','#ef4444']}]},options:{responsive:true,maintainAspectRatio:false,plugins:{legend:{position:'bottom',labels:{boxWidth:12,font:{size:11}}}}}});
    }
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadPerilaku(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/perilaku',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Belum ada catatan perilaku</div>';return}
    const pos=d.filter(x=>x.kategori==='positif');
    const neg=d.filter(x=>x.kategori==='negatif');
    let html='<div class="card"><div class="card-content">';
    html+='<div style="display:flex;gap:12px;margin-bottom:16px;text-align:center">';
    html+='<div style="flex:1;padding:12px;background:#dcfce7;border-radius:var(--radius)"><div style="font-size:22px;font-weight:700;color:#166534">'+pos.length+'</div><div style="font-size:11px;color:#166534;font-weight:500">Positif</div></div>';
    html+='<div style="flex:1;padding:12px;background:#fef2f2;border-radius:var(--radius)"><div style="font-size:22px;font-weight:700;color:#991b1b">'+neg.length+'</div><div style="font-size:11px;color:#991b1b;font-weight:500">Negatif</div></div>';
    html+='</div></div></div>';
    html+='<div class="card"><div class="card-header"><h1 style="font-size:15px">Riwayat Catatan</h1></div><div class="card-content">';
    html+=d.map(n=>{
      const dt=new Date(n.tanggal);
      const dateStr=dt.toLocaleDateString('id-ID',{day:'numeric',month:'short',year:'numeric'});
      const cls=n.kategori==='positif'?'badge-success':'badge-destructive';
      const icon=n.kategori==='positif'?'✅':'⚠️';
      return'<div style="padding:10px 0;border-bottom:1px solid var(--border)"><div style="display:flex;align-items:center;gap:8px;margin-bottom:4px"><span class="badge '+cls+'">'+icon+' '+esc(n.kategori.charAt(0).toUpperCase()+n.kategori.slice(1))+'</span><span style="font-size:11px;color:var(--muted-foreground)">'+dateStr+'</span></div><div style="font-size:13px">'+esc(n.deskripsi)+'</div></div>'
    }).join('');
    html+='</div></div>';
    c.innerHTML=html;
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadUjian(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/ujian-skor',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Belum ada data ujian</div>';return}
    c.innerHTML='<div class="table-wrap"><table class="table"><thead><tr><th>Ujian</th><th>Mapel</th><th>Skor</th><th>Tanggal</th></tr></thead><tbody>'+
    d.map(p=>'<tr><td>'+esc(p.ujian?.judul||'-')+'</td><td>'+esc(p.ujian?.mapel?.namaMapel||'-')+'</td><td><strong>'+(p.skor!=null?Number(p.skor).toFixed(1):'-')+'</strong></td><td>'+String(p.selesai||'').slice(0,10)+'</td></tr>').join('')+
    '</tbody></table></div>';
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadTugas(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/tugas',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Belum ada tugas</div>';return}
    c.innerHTML='<div class="table-wrap"><table class="table"><thead><tr><th>Judul</th><th>Mapel</th><th>Deadline</th><th>Status</th><th>Nilai</th></tr></thead><tbody>'+
    d.map(t=>{
      const dl=new Date(t.deadline);const now=new Date();const overdue=dl<now&&t.statusPengumpulan==='belum';
      const cls=t.statusPengumpulan==='Dinilai'?'badge-success':t.statusPengumpulan==='Dikumpulkan'?'badge-warning':overdue?'badge-destructive':'badge-secondary';
      const label=t.statusPengumpulan==='Dinilai'?'Dinilai':t.statusPengumpulan==='Dikumpulkan'?'Dikumpulkan':overdue?'Terlambat':'Belum';
      return'<tr><td>'+esc(t.judul)+'</td><td>'+esc(t.mapel?.namaMapel||'-')+'</td><td>'+dl.toLocaleDateString('id-ID')+'</td><td><span class="badge '+cls+'">'+label+'</span></td><td>'+(t.nilai!=null?Number(t.nilai).toFixed(0):'-')+'</td></tr>'
    }).join('')+
    '</tbody></table></div>';
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadMateri(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/materi',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Belum ada materi</div>';return}
    c.innerHTML=d.map(m=>'<div style="padding:10px 0;border-bottom:1px solid var(--border)"><div style="font-weight:600;font-size:13px">'+esc(m.judul)+'</div><div style="font-size:12px;color:var(--muted-foreground)">'+esc(m.mapel?.namaMapel||'')+' &middot; '+String(m.createdAt||'').slice(0,10)+'</div></div>').join('');
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadKalender(c){
  try{
    const r=await fetch(API+'/kalender',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Tidak ada event</div>';return}
    c.innerHTML=d.map(e=>'<div style="padding:10px 0;border-bottom:1px solid var(--border)"><div style="font-weight:600;font-size:13px">'+esc(e.judul)+'</div><div style="font-size:12px;color:var(--muted-foreground)">'+String(e.tanggalMulai||'').slice(0,10)+(e.tanggalSelesai?' s/d '+String(e.tanggalSelesai).slice(0,10):'')+'</div><div style="font-size:12px;margin-top:4px">'+esc(e.deskripsi||'')+'</div></div>').join('');
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadNotif(c){
  try{
    const r=await fetch(API+'/notifikasi',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Tidak ada notifikasi</div>';return}
    c.innerHTML=d.map(n=>'<div class="notif-item'+(n.isRead?'':'')+'">'+(n.isRead?'':'<span class="notif-dot"></span>')+'<div class="notif-title">'+esc(n.judul)+'</div><div>'+esc(n.isi)+'</div><div class="notif-time">'+String(n.createdAt||'').slice(0,16).replace('T',' ')+'</div></div>').join('');
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function loadChat(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/chat',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    const uid=JSON.parse(atob(state.token.split('.')[1])).sub;
    let html='<div class="card"><div class="card-content"><div class="chat-messages" id="chatMsgs">';
    if(Array.isArray(d)&&d.length){
      html+=d.map(m=>'<div class="chat-msg '+(m.pengirimUserId===uid?'sent':'received')+'">'+esc(m.isi)+'</div>').join('');
    }else{
      html+='<div class="empty-state">Mulai chat dengan wali kelas</div>';
    }
    html+='</div><div class="chat-input-wrap"><input class="input" id="chatInput" placeholder="Ketik pesan..." onkeydown="if(event.key==='Enter')sendChat()"><button class="btn btn-primary" onclick="sendChat()">Kirim</button></div></div></div>';
    c.innerHTML=html;
    const el=document.getElementById('chatMsgs');if(el)el.scrollTop=el.scrollHeight;
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

async function sendChat(){
  const input=document.getElementById('chatInput');
  const isi=input?input.value.trim():'';
  if(!isi)return;
  input.value='';
  const uid=JSON.parse(atob(state.token.split('.')[1])).sub;
  const msgs=document.getElementById('chatMsgs');
  if(msgs){msgs.innerHTML+='<div class="chat-msg sent">'+esc(isi)+'</div>';msgs.scrollTop=msgs.scrollHeight}
  try{
    await fetch(API+'/orang-tua/anak/'+state.anakId+'/chat',{method:'POST',headers:{...hdr(),'Content-Type':'application/json'},body:JSON.stringify({isi})});
  }catch(e){}
}

async function loadBuku(c){
  try{
    const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/peminjaman',{headers:hdr()});
    const d=await r.json();if(!r.ok)throw new Error(d.error);
    if(!Array.isArray(d)||!d.length){c.innerHTML='<div class="empty-state">Tidak ada peminjaman buku</div>';return}
    c.innerHTML='<div class="table-wrap"><table class="table"><thead><tr><th>Judul</th><th>Tgl Pinjam</th><th>Deadline</th><th>Status</th></tr></thead><tbody>'+
    d.map(p=>{
      const cls=p.status==='Dipinjam'?'badge-warning':'badge-success';
      return'<tr><td>'+esc(p.buku?.judul||'-')+'</td><td>'+String(p.tanggalPinjam||'').slice(0,10)+'</td><td>'+String(p.tanggalJatuhTempo||'').slice(0,10)+'</td><td><span class="badge '+cls+'">'+esc(p.status)+'</span></td></tr>'
    }).join('')+
    '</tbody></table></div>';
  }catch(e){c.innerHTML='<div class="error-box show">'+esc(e.message)+'</div>'}
}

function doLogout(){state.token='';resetTurnstile();hide(document.getElementById('portalCard'));show(document.getElementById('loginCard'))}
function showErr(id,msg){const e=document.getElementById(id);e.textContent=msg;show(e)}
function hideErr(id){document.getElementById(id).classList.remove('show')}
</script>
</body>
</html>`
