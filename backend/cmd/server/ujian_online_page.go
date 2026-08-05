package main

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) serveUjianOnlinePage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(ujianOnlineHTML)
}

var ujianOnlineHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ujian Online — PKBM Tunas Ilmu</title>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
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
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:var(--background);color:var(--foreground);min-height:100vh;-webkit-font-smoothing:antialiased}
.wrap{max-width:480px;margin:0 auto;padding:24px 16px}

.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius);box-shadow:0 1px 2px 0 rgb(0 0 0 / 0.05);overflow:hidden}
.card-header{padding:24px 24px 0}
.card-content{padding:24px}
.card-footer{padding:0 24px 24px;display:flex;gap:8px}

h1{font-size:18px;font-weight:600;letter-spacing:-0.025em;margin:0}
p.desc{font-size:14px;color:var(--muted-foreground);margin-top:1.5px}

.form-group{margin-bottom:16px}
.form-group:last-child{margin-bottom:0}
label{display:block;font-size:14px;font-weight:500;margin-bottom:6px;color:var(--foreground)}
.input{width:100%;height:36px;padding:0 12px;border:1px solid var(--input);border-radius:var(--radius);font-size:14px;font-family:inherit;background:var(--background);color:var(--foreground);transition:border-color .15s,box-shadow .15s}
.input:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}
.input::placeholder{color:var(--muted-foreground)}
.input-mono{font-family:'SF Mono',SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:0.15em;font-size:16px;text-transform:uppercase}

.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;white-space:nowrap;height:36px;padding:0 16px;border-radius:var(--radius);font-size:14px;font-weight:500;font-family:inherit;cursor:pointer;transition:background .15s,border-color .15s,color .15s;border:1px solid transparent;text-decoration:none}
.btn:disabled{opacity:.5;pointer-events:none}
.btn-primary{background:var(--primary);color:var(--primary-foreground);border-color:var(--primary)}
.btn-primary:hover{background:#27272a}
.btn-outline{background:var(--background);color:var(--foreground);border-color:var(--input)}
.btn-outline:hover{background:var(--accent);color:var(--accent-foreground)}
.btn-destructive{background:var(--destructive);color:var(--destructive-foreground)}
.btn-destructive:hover{opacity:.9}
.btn-ghost{background:transparent;color:var(--foreground);border-color:transparent}
.btn-ghost:hover{background:var(--accent);color:var(--accent-foreground)}
.btn-success{background:var(--success);color:#fff;border-color:var(--success)}
.btn-success:hover{opacity:.9}
.btn-sm{height:32px;padding:0 12px;font-size:13px}
.btn-lg{height:40px;padding:0 24px}

.error-box{background:#fef2f2;border:1px solid #fecaca;color:var(--destructive);padding:10px 14px;border-radius:var(--radius);font-size:13px;margin-bottom:16px;display:none}
.error-box.show{display:block}

.timer{font-size:28px;font-weight:700;font-variant-numeric:tabular-nums;text-align:center;padding:12px;border-radius:var(--radius);margin-bottom:16px;background:var(--secondary);color:var(--foreground);letter-spacing:-0.025em}
.timer.warning{color:var(--warning);background:#fefce8;border:1px solid #fef08a}
.timer.danger{color:var(--destructive);background:#fef2f2;border:1px solid #fecaca}

.progress{height:6px;background:var(--secondary);border-radius:3px;margin-bottom:16px;overflow:hidden}
.progress-bar{height:100%;background:var(--primary);border-radius:3px;transition:width .3s}

.badge{display:inline-flex;align-items:center;border-radius:9999px;padding:2px 8px;font-size:11px;font-weight:600;line-height:1}
.badge-default{background:var(--primary);color:var(--primary-foreground)}
.badge-secondary{background:var(--secondary);color:var(--secondary-foreground);border:1px solid var(--border)}
.badge-success{background:#dcfce7;color:#166534}
.badge-outline{background:transparent;color:var(--foreground);border:1px solid var(--border)}

.exam-item{border:1px solid var(--border);border-radius:var(--radius);padding:16px;margin-bottom:8px;transition:border-color .15s,box-shadow .15s}
.exam-item:hover{border-color:#a1a1aa;box-shadow:0 1px 3px 0 rgb(0 0 0 / 0.1)}
.exam-item h3{font-size:15px;font-weight:600;margin:0 0 4px}
.exam-meta{font-size:13px;color:var(--muted-foreground);display:flex;flex-wrap:wrap;gap:4px 12px;margin:0 0 12px}

.question-card{border:1px solid var(--border);border-radius:var(--radius);padding:20px;margin-bottom:8px}
.question-num{font-size:13px;font-weight:600;color:var(--muted-foreground);text-transform:uppercase;letter-spacing:0.05em;margin-bottom:8px}
.question-text{font-size:15px;line-height:1.6;margin-bottom:16px;color:var(--foreground)}

.option{display:flex;align-items:center;gap:12px;padding:12px 14px;border:1px solid var(--border);border-radius:var(--radius);margin-bottom:6px;cursor:pointer;transition:all .15s;background:var(--background)}
.option:hover{border-color:#a1a1aa;background:var(--secondary)}
.option.selected{border-color:var(--primary);background:var(--secondary);box-shadow:0 0 0 1px var(--primary)}
.option input[type="radio"]{width:16px;height:16px;margin:0;accent-color:var(--primary);flex-shrink:0}
.option-label{font-size:14px;line-height:1.5}

.textarea{width:100%;min-height:120px;padding:12px;border:1px solid var(--input);border-radius:var(--radius);font-size:14px;font-family:inherit;resize:vertical;background:var(--background);color:var(--foreground);transition:border-color .15s,box-shadow .15s}
.textarea:focus{outline:none;border-color:var(--ring);box-shadow:0 0 0 2px rgba(24,24,27,.1)}

.nav-buttons{display:flex;gap:8px;margin-top:16px}

.result-box{text-align:center;padding:32px 16px}
.result-score{font-size:56px;font-weight:700;color:var(--foreground);letter-spacing:-0.025em;line-height:1}
.result-label{font-size:14px;color:var(--muted-foreground);margin-top:4px}
.result-detail{margin-top:20px;font-size:14px;color:var(--muted-foreground);line-height:1.8}
.result-detail strong{color:var(--foreground);font-weight:600}

.hidden{display:none!important}
@media(max-width:640px){.wrap{padding:12px 8px}.card-content{padding:16px}.card-header{padding:16px 16px 0}.card-footer{padding:0 16px 16px}.timer{font-size:22px}}
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

<script>
const API='/api';
let state={nisn:'',aksesKode:'',ujians:[],currentUjian:null,ujianPesertaId:'',soal:[],jawaban:{},currentIdx:0,timerInterval:null,sisaWaktu:0};

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
startTimer(d.mulai);
}catch(e){alert(e.message)}
}

async function loadSoal(ujianId){
const r=await fetch(API+'/ujian-online/'+ujianId+'/soal?nisn='+encodeURIComponent(state.nisn)+'&aksesKode='+encodeURIComponent(state.aksesKode));
const d=await r.json();if(!r.ok)throw new Error(d.error||'Gagal');
state.soal=d.soal||[];state.sisaWaktu=d.sisaWaktu||0;
(d.jawaban||[]).forEach(j=>{state.jawaban[j.ujianSoalId]=j.jawaban});
state.currentIdx=0;renderSoal();
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
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);fd.append('ujianSoalId',soalId);fd.append('jawaban',String(val));
fetch(API+'/ujian-online/'+state.currentUjian.id+'/jawab',{method:'POST',body:fd}).catch(()=>{});
renderSoal();
}
function jawabTeks(soalId,val){state.jawaban[soalId]=val;
const fd=new FormData();fd.append('nisn',state.nisn);fd.append('aksesKode',state.aksesKode);fd.append('ujianSoalId',soalId);fd.append('jawaban',val);
fetch(API+'/ujian-online/'+state.currentUjian.id+'/jawab',{method:'POST',body:fd}).catch(()=>{});
}
function prevSoal(){if(state.currentIdx>0){state.currentIdx--;renderSoal()}}
function nextSoal(){if(state.currentIdx<state.soal.length-1){state.currentIdx++;renderSoal()}}

function startTimer(mulaiStr){
if(state.timerInterval)clearInterval(state.timerInterval);
const durasi=state.currentUjian?.durasiMenit||60;
const mulai=new Date(mulaiStr).getTime();
const batas=mulai+durasi*60*1000;
state.timerInterval=setInterval(()=>{
const sisa=Math.max(0,Math.floor((batas-Date.now())/1000));
const h=Math.floor(sisa/3600),m=Math.floor((sisa%3600)/60),s=sisa%60;
const el=document.getElementById('timer');
el.textContent=String(h).padStart(2,'0')+':'+String(m).padStart(2,'0')+':'+String(s).padStart(2,'0');
el.className='timer'+(sisa<300?' danger':sisa<600?' warning':'');
if(sisa<=0){clearInterval(state.timerInterval);selesaiUjian(true)}
},1000);
}

async function selesaiUjian(auto){
if(!auto&&!confirm('Yakin ingin menyelesaikan ujian?'))return;
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
fetch(API+'/ujian-online/'+state.currentUjian.id+'/tab-switch',{method:'POST',body:fd}).catch(()=>{});
}
});

function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
</script>
</body>
</html>`
