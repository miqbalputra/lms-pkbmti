package main

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) serveUjianOnlinePage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	siteKey := os.Getenv("TURNSTILE_SITE_KEY")
	html := strings.Replace(ujianOnlineHTML, "{{TURNSTILE_SITE_KEY}}", siteKey, 1)
	return c.SendString(html)
}

var ujianOnlineHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<meta name="theme-color" content="#ffffff">
<title>Ujian Online — PKBM Tunas Ilmu</title>
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

.question-card{border:1px solid var(--border);border-radius:var(--radius);padding:20px 16px;margin-bottom:8px}
.question-num{font-size:12px;font-weight:600;color:var(--muted-foreground);text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px}
.question-text{font-size:15px;line-height:1.7;margin-bottom:16px;color:var(--foreground);word-break:break-word}

.option{display:flex;align-items:flex-start;gap:12px;padding:14px;border:1px solid var(--border);border-radius:var(--radius);margin-bottom:8px;cursor:pointer;transition:all .15s;background:var(--background);min-height:48px}
.option:hover{border-color:#a1a1aa;background:var(--secondary)}
.option.selected{border-color:var(--primary);background:var(--secondary);box-shadow:0 0 0 1px var(--primary)}
.option input[type="radio"]{width:18px;height:18px;margin:1px 0 0 0;accent-color:var(--primary);flex-shrink:0}
.option-label{font-size:15px;line-height:1.5}

.textarea{width:100%;min-height:140px;padding:14px;border:1px solid var(--input);border-radius:var(--radius);font-size:16px;font-family:inherit;resize:vertical;background:var(--background);color:var(--foreground);transition:border-color .15s,box-shadow .15s;-webkit-appearance:none;appearance:none}
.textarea:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}

.nav-buttons{display:flex;gap:8px;margin-top:16px}

.result-box{text-align:center;padding:32px 16px}
.result-score{font-size:56px;font-weight:700;color:var(--foreground);letter-spacing:-0.025em;line-height:1}
.result-label{font-size:14px;color:var(--muted-foreground);margin-top:4px}
.result-detail{margin-top:20px;font-size:15px;color:var(--muted-foreground);line-height:1.8}
.result-detail strong{color:var(--foreground);font-weight:600}

.hidden{display:none!important}

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
}
</style>
</head>
<body>
<div class="wrap">

<!-- Login -->
<div id="loginCard" class="card">
  <div class="card-header"><h1>Masuk Ujian Online</h1><p class="desc">Masukkan NISN dan kode akses dari guru Anda.</p></div>
  <div class="card-content">
    <div id="loginError" class="error-box"></div>
    <div class="form-group">
      <label for="nisn">NISN</label>
      <input class="input" type="text" id="nisn" placeholder="Nomor Induk Siswa Nasional" maxlength="20" autocomplete="off">
    </div>
    <div class="form-group">
      <label for="aksesKode">Kode Akses</label>
      <input class="input input-mono" type="text" id="aksesKode" placeholder="XXXXXX" maxlength="6" autocomplete="off" style="text-transform:uppercase">
    </div>
    <div class="form-group" id="turnstileContainer">
      <div class="cf-turnstile" data-sitekey="{{TURNSTILE_SITE_KEY}}" data-theme="light" data-callback="onTurnstileSuccess"></div>
    </div>
  </div>
  <div class="card-footer">
    <button class="btn btn-primary btn-lg" onclick="cekUjian()" id="cekBtn" style="width:100%">Cari Ujian</button>
  </div>
</div>

<!-- Exam List -->
<div id="listCard" class="card hidden">
  <div class="card-header"><h1>Daftar Ujian</h1><p class="desc">NISN: <strong id="displayNisn"></strong></p></div>
  <div class="card-content">
    <div id="examList"></div>
  </div>
  <div class="card-footer">
    <button class="btn btn-outline" onclick="showLogin()" style="width:100%">Ganti Akun</button>
  </div>
</div>

<!-- Exam Taking -->
<div id="examCard" class="card hidden">
  <div class="card-header">
    <div style="display:flex;align-items:center;justify-content:space-between">
      <h1 id="examTitle">Ujian</h1>
      <span class="badge badge-secondary" id="soalBadge">0/0</span>
    </div>
  </div>
  <div class="card-content">
    <div class="timer" id="timer">00:00:00</div>
    <div class="progress"><div class="progress-bar" id="progressBar" style="width:0%"></div></div>
    <div id="soalContainer"></div>
    <div class="nav-buttons">
      <button class="btn btn-outline" onclick="prevSoal()" id="prevBtn" disabled style="flex:0 0 auto">Sebelumnya</button>
      <button class="btn btn-primary" onclick="nextSoal()" id="nextBtn" style="flex:1">Selanjutnya</button>
      <button class="btn btn-success hidden" onclick="selesaiUjian()" id="selesaiBtn" style="flex:1">Selesai</button>
    </div>
  </div>
</div>

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
    <button class="btn btn-primary" onclick="showLogin()">Kembali ke Awal</button>
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
    <button class="btn btn-primary btn-lg" onclick="reconnect()" id="reconnectBtn" style="width:100%">Sambungkan Ulang</button>
  </div>
</div>

<!-- Connection Restored Toast -->
<div id="toastReconnect" style="position:fixed;top:16px;left:50%;transform:translateX(-50%);background:var(--success);color:#fff;padding:12px 20px;border-radius:var(--radius);font-size:14px;font-weight:500;z-index:10000;box-shadow:0 4px 12px rgba(0,0,0,.15);display:none">Koneksi tersambung kembali</div>

<script>
const API='/api';
let state={nisn:'',aksesKode:'',ujians:[],currentUjian:null,ujianPesertaId:'',soal:[],jawaban:{},currentIdx:0,timerInterval:null,sisaWaktu:0,mulai:null,offlineQueue:[]};

// --- Connectivity Detection ---
let isOnline=navigator.onLine;
function updateOnlineStatus(){
  const wasOnline=isOnline;isOnline=navigator.onLine;
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
  // Sync queued jawaban
  while(state.offlineQueue.length){
    const j=state.offlineQueue.shift();sendJawaban(j.soalId,j.val);
  }
  // Reload soal to sync with server
  if(state.currentUjian){loadSoal(state.currentUjian.id).catch(()=>{})}
}

async function reconnect(){
  const btn=document.getElementById('reconnectBtn');btn.disabled=true;btn.textContent='Menyambungkan...';
  try{
    // Test connection
    const r=await fetch(API+'/health');if(!r.ok)throw new Error();
    // If in exam, reload soal
    if(state.currentUjian){
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
function showLogin(){show(document.getElementById('loginCard'));hide(document.getElementById('listCard'));hide(document.getElementById('examCard'));hide(document.getElementById('resultCard'));clearInterval(state.timerInterval)}
function showError(id,msg){const e=document.getElementById(id);e.textContent=msg;show(e)}

async function cekUjian(){
const nisn=document.getElementById('nisn').value.trim();
const kode=document.getElementById('aksesKode').value.trim();
if(!nisn||!kode){showError('loginError','NISN dan Kode Akses wajib diisi.');return}
document.getElementById('loginError').classList.remove('show');
document.getElementById('cekBtn').disabled=true;document.getElementById('cekBtn').textContent='Mencari...';
try{
const fd=new FormData();fd.append('nisn',nisn);fd.append('aksesKode',kode);
const tw=document.querySelector('[name="cf-turnstile-response"]');
if(tw)fd.append('cf-turnstile-response',tw.value);
const r=await fetch(API+'/ujian-online/cek',{method:'POST',body:fd});
const d=await r.json();
if(!r.ok)throw new Error(d.error||'Gagal');
state.nisn=nisn;state.aksesKode=kode;state.ujians=d;
document.getElementById('displayNisn').textContent=nisn;
renderExamList();
hide(document.getElementById('loginCard'));show(document.getElementById('listCard'));
}catch(e){showError('loginError',e.message)}
finally{document.getElementById('cekBtn').disabled=false;document.getElementById('cekBtn').textContent='Cari Ujian'}
}

function renderExamList(){
const c=document.getElementById('examList');
if(!state.ujians.length){c.innerHTML='<p style="color:var(--muted-foreground);font-size:14px;text-align:center;padding:24px 0">Tidak ada ujian aktif untuk kode ini.</p>';return}
c.innerHTML=state.ujians.map(u=>{
const mulai=new Date(u.waktuMulai).toLocaleString('id-ID',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});
const selesai=new Date(u.waktuSelesai).toLocaleString('id-ID',{day:'numeric',month:'short',year:'numeric',hour:'2-digit',minute:'2-digit'});
let badge=u.sudahMengerjakan?'<span class="badge badge-success">Selesai</span>':'';
return '<div class="exam-item"><h3>'+esc(u.judul)+'</h3><div class="exam-meta"><span>'+esc(u.mapel?.namaMapel||'')+'</span><span>'+u.durasiMenit+' menit</span><span>'+mulai+' — '+selesai+'</span></div><div style="display:flex;align-items:center;gap:8px">'+badge+'<button class="btn btn-primary btn-sm" onclick="mulaiUjian(\''+u.id+'\')" '+(u.sudahMengerjakan?'disabled':'')+'>Mulai</button></div></div>'
}).join('');
}

async function mulaiUjian(ujianId){
try{
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);
const r=await fetch(API+'/ujian-online/'+ujianId+'/mulai',{method:'POST',body:fd});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
state.currentUjian=state.ujians.find(u=>u.id===ujianId);
state.ujianPesertaId=d.id;
await loadSoal(ujianId);
hide(document.getElementById('listCard'));show(document.getElementById('examCard'));
document.getElementById('examTitle').textContent=state.currentUjian?.judul||'Ujian';
state.currentUjian.gracePeriodMenit=d.gracePeriodMenit||5;
startTimer(d.mulai,d.sisaWaktu);
}catch(e){alert(e.message)}
}

async function loadSoal(ujianId){
const r=await fetch(API+'/ujian-online/'+ujianId+'/soal?nisn='+encodeURIComponent(state.nisn)+'&aksesKode='+encodeURIComponent(state.aksesKode));
const d=await r.json();if(!r.ok){
  if(d.error&&d.error.includes('habis')){hide(document.getElementById('examCard'));show(document.getElementById('resultCard'));document.getElementById('scoreValue').textContent='—';document.getElementById('scoreDetail').innerHTML='<strong>Waktu ujian sudah habis.</strong>';}
  throw new Error(d.error||'Gagal');
}
state.soal=d.soal||[];state.sisaWaktu=d.sisaWaktu||0;
state.currentUjian=state.currentUjian||{};
state.currentUjian.gracePeriodMenit=d.gracePeriodMenit||5;
(d.jawaban||[]).forEach(j=>{state.jawaban[j.ujianSoalId]=j.jawaban});
state.currentIdx=0;renderSoal();
// Restart timer with server time (handles reconnect / grace period)
if(state.sisaWaktu!==0){startTimer(null,state.sisaWaktu)}
}

function renderSoal(){
const total=state.soal.length;if(!total)return;
const s=state.soal[state.currentIdx];
document.getElementById('soalBadge').textContent=(state.currentIdx+1)+'/'+total;
document.getElementById('progressBar').style.width=((state.currentIdx+1)/total*100)+'%';
const c=document.getElementById('soalContainer');
let html='<div class="question-card"><div class="question-num">Soal '+state.currentIdx+' dari '+total+'</div><div class="question-text">'+esc(s.pertanyaan)+'</div>';
if(s.tipe==='pg'&&s.opsi){
s.opsi.forEach((op,i)=>{
const sel=state.jawaban[s.id]===String(i)?'selected':'';
html+='<label class="option '+sel+'" onclick="jawab(\''+s.id+'','+i+')"><input type="radio" name="soal_'+s.id+'" '+(sel?'checked':'')+'><span class="option-label"><strong>'+String.fromCharCode(65+i)+'</strong>. '+esc(op)+'</span></label>';
});
}else{
html+='<textarea class="textarea" oninput="jawabTeks(\''+s.id+'',this.value)" placeholder="Tulis jawaban Anda di sini...">'+esc(state.jawaban[s.id]||'')+'</textarea>';
}
html+='</div>';
c.innerHTML=html;
document.getElementById('prevBtn').disabled=state.currentIdx===0;
document.getElementById('nextBtn').classList.toggle('hidden',state.currentIdx>=total-1);
document.getElementById('selesaiBtn').classList.toggle('hidden',state.currentIdx<total-1);
}

function jawab(soalId,val){
state.jawaban[soalId]=String(val);
sendJawaban(soalId,String(val));
renderSoal();
}
function jawabTeks(soalId,val){state.jawaban[soalId]=val;
sendJawaban(soalId,val);
}
function sendJawaban(soalId,val){
if(!isOnline){state.offlineQueue.push({soalId,val});return}
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);fd.append('ujianSoalId',soalId);fd.append('jawaban',val);
fetch(API+'/ujian-online/'+state.currentUjian.id+'/jawab',{method:'POST',body:fd}).catch(()=>{state.offlineQueue.push({soalId,val})});
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
el.className='timer'+(sisa<300?' danger':sisa<600?' warning':'');
if(sisa<=0){inGrace=true}// switch to grace mode
}else{
// Grace period countdown
const sisaG=Math.max(0,Math.floor((batasGrace-now)/1000));
const m=Math.floor((sisaG%3600)/60),ss=sisaG%60;
const el=document.getElementById('timer');
el.textContent='GRACE '+String(m).padStart(2,'0')+':'+String(ss).padStart(2,'0');
el.className='timer danger';
if(sisaG<=0){clearInterval(state.timerInterval);selesaiUjian(true)}
}
},1000);
}

async function selesaiUjian(auto){
if(!auto&&!confirm('Yakin ingin menyelesaikan ujian?'))return;
if(!isOnline){showOfflinePopup();return}
clearInterval(state.timerInterval);
try{
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);
const r=await fetch(API+'/ujian-online/'+state.currentUjian.id+'/selesai',{method:'POST',body:fd});
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
document.getElementById('scoreValue').textContent=Math.round(d.skor||0);
document.getElementById('scoreDetail').innerHTML='Benar: <strong>'+d.benar+'</strong> dari <strong>'+d.total+'</strong> soal<br>Status: <strong>Selesai</strong>';
hide(document.getElementById('examCard'));show(document.getElementById('resultCard'));
}catch(e){alert(e.message)}
}

document.addEventListener('visibilitychange',()=>{
if(document.hidden&&state.currentUjian&&!document.getElementById('examCard').classList.contains('hidden')){
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);
fetch(API+'/ujian-online/'+state.currentUjian.id+'/tab-switch',{method:'POST',body:fd}).then(r=>r.json()).then(d=>{
  if(d.locked){
    if(state.timerInterval)clearInterval(state.timerInterval);
    document.getElementById('scoreValue').textContent=Math.round(d.skor||0);
    document.getElementById('scoreDetail').innerHTML='Ujian dikunci (terlalu sering pindah tab).<br>Skor: <strong>'+Math.round(d.skor||0)+'</strong>';
    hide(document.getElementById('examCard'));show(document.getElementById('resultCard'));
  }
}).catch(()=>{});
}
});

function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
</script>
</body>
</html>`
