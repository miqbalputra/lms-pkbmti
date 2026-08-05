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
<style>
:root{--brand:#1c5740;--gold:#d4af37;--bg:#f5f7f6;--card:#fff;--border:#e5e7eb;--text:#222;--muted:#666;--success:#16a34a;--danger:#dc2626;--warning:#f59e0b}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,Segoe UI,Roboto,Arial,sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.wrap{max-width:800px;margin:0 auto;padding:16px}
.card{background:var(--card);border:1px solid var(--border);border-radius:16px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.06);margin-bottom:16px}
.head{background:var(--brand);color:#fff;padding:20px 24px}
.head .org{font-size:12px;letter-spacing:.12em;text-transform:uppercase;opacity:.85}
.head .name{font-size:20px;font-weight:700;margin-top:2px}
.gold{height:3px;background:var(--gold)}
.pad{padding:24px}
h1{font-size:20px;margin:0 0 16px}
h2{font-size:17px;margin:0 0 12px}
label{display:block;font-size:13px;font-weight:600;margin-bottom:4px;color:var(--muted)}
input,select{width:100%;padding:10px 14px;border:1px solid var(--border);border-radius:10px;font-size:14px;margin-bottom:12px}
input:focus{outline:none;border-color:var(--brand);box-shadow:0 0 0 3px rgba(28,87,64,.1)}
.btn{display:inline-flex;align-items:center;gap:8px;padding:10px 20px;border-radius:10px;border:none;font-weight:600;font-size:14px;cursor:pointer;transition:all .15s}
.btn-primary{background:var(--brand);color:#fff}.btn-primary:hover{opacity:.9}
.btn-success{background:var(--success);color:#fff}.btn-success:hover{opacity:.9}
.btn-danger{background:var(--danger);color:#fff}.btn-danger:hover{opacity:.9}
.btn-outline{background:#fff;border:1px solid var(--border);color:var(--text)}.btn-outline:hover{background:#f9fafb}
.btn:disabled{opacity:.5;cursor:not-allowed}
.error{background:#fef2f2;border:1px solid #fecaca;color:var(--danger);padding:10px 14px;border-radius:10px;font-size:13px;margin-bottom:12px;display:none}
.error.show{display:block}
.timer{font-size:24px;font-weight:700;color:var(--brand);text-align:center;padding:12px;background:#f0fdf4;border-radius:10px;margin-bottom:16px}
.timer.warning{color:var(--warning);background:#fffbeb}
.timer.danger{color:var(--danger);background:#fef2f2}
@media(max-width:640px){
  .wrap{padding:8px}
  .pad{padding:16px}
  h1{font-size:18px}
  .timer{font-size:20px;padding:10px}
  .option{padding:8px 10px;font-size:13px}
  .btn{padding:8px 14px;font-size:13px}
}
.exam-item{border:1px solid var(--border);border-radius:12px;padding:16px;margin-bottom:12px;transition:all .15s}
.exam-item:hover{border-color:var(--brand);box-shadow:0 2px 8px rgba(0,0,0,.08)}
.exam-item h3{font-size:16px;margin:0 0 4px}
.exam-item .meta{font-size:13px;color:var(--muted);margin:0 0 12px}
.exam-item .actions{display:flex;gap:8px;align-items:center}
.badge{display:inline-block;padding:2px 8px;border-radius:6px;font-size:11px;font-weight:600}
.badge-done{background:#dcfce7;color:var(--success)}
.badge-active{background:#dbeafe;color:#2563eb}
.question{border:1px solid var(--border);border-radius:12px;padding:16px;margin-bottom:12px}
.question .num{font-weight:700;color:var(--brand);margin-bottom:8px}
.question .text{font-size:15px;line-height:1.6;margin-bottom:12px}
.option{display:flex;align-items:center;gap:10px;padding:10px 14px;border:1px solid var(--border);border-radius:10px;margin-bottom:6px;cursor:pointer;transition:all .15s}
.option:hover{border-color:var(--brand);background:#f0fdf4}
.option.selected{border-color:var(--brand);background:#dcfce7}
.option input{margin:0}
.progress{height:6px;background:var(--border);border-radius:3px;margin-bottom:16px;overflow:hidden}
.progress-bar{height:100%;background:var(--brand);border-radius:3px;transition:width .3s}
.hidden{display:none}
.result-box{text-align:center;padding:32px}
.result-box .score{font-size:48px;font-weight:800;color:var(--brand)}
.result-box .label{font-size:14px;color:var(--muted);margin-top:4px}
.result-box .detail{margin-top:16px;font-size:15px;line-height:1.8}
</style>
</head>
<body>
<div class="wrap">
<!-- Login Form -->
<div id="loginCard" class="card">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Ujian Online</div></div>
<div class="gold"></div>
<div class="pad">
<h1>Masuk Ujian Online</h1>
<div id="loginError" class="error"></div>
<div id="loginForm">
<label>NISN (Nomor Induk Siswa Nasional)</label>
<input type="text" id="nisn" placeholder="Masukkan NISN" maxlength="20" autocomplete="off">
<label>Kode Akses Ujian</label>
<input type="text" id="aksesKode" placeholder="Masukkan kode akses dari guru" maxlength="50" autocomplete="off">
<button class="btn btn-primary" onclick="cekUjian()" id="cekBtn">Cari Ujian</button>
</div>
</div>
</div>

<!-- Exam List -->
<div id="listCard" class="card hidden">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Daftar Ujian</div></div>
<div class="gold"></div>
<div class="pad">
<h1>Ujian Tersedia</h1>
<p style="color:var(--muted);font-size:13px;margin-bottom:16px">NISN: <strong id="displayNisn"></strong></p>
<div id="examList"></div>
<button class="btn btn-outline" onclick="showLogin()" style="margin-top:8px">Ganti Akun</button>
</div>
</div>

<!-- Exam Taking -->
<div id="examCard" class="card hidden">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name" id="examTitle">Ujian</div></div>
<div class="gold"></div>
<div class="pad">
<div class="timer" id="timer">00:00:00</div>
<div class="progress"><div class="progress-bar" id="progressBar" style="width:0%"></div></div>
<p style="font-size:13px;color:var(--muted);margin-bottom:12px">Soal <span id="soalNum">0</span> / <span id="soalTotal">0</span></p>
<div id="soalContainer"></div>
<div style="display:flex;gap:8px;margin-top:16px">
<button class="btn btn-outline" onclick="prevSoal()" id="prevBtn" disabled>Sebelumnya</button>
<button class="btn btn-primary" onclick="nextSoal()" id="nextBtn">Selanjutnya</button>
<button class="btn btn-success hidden" onclick="selesaiUjian()" id="selesaiBtn">Selesai Ujian</button>
</div>
</div>
</div>

<!-- Result -->
<div id="resultCard" class="card hidden">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Hasil Ujian</div></div>
<div class="gold"></div>
<div class="pad result-box">
<div class="score" id="scoreValue">0</div>
<div class="label">Nilai Anda</div>
<div class="detail" id="scoreDetail"></div>
<button class="btn btn-primary" onclick="showLogin()" style="margin-top:24px">Kembali ke Awal</button>
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
if(!nisn||!kode){showError('loginError','NISN dan Kode Akses wajib diisi');return}
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
if(!state.ujians.length){c.innerHTML='<p style="color:var(--muted)">Tidak ada ujian aktif.</p>';return}
c.innerHTML=state.ujians.map(u=>{
const mulai=new Date(u.waktuMulai).toLocaleString('id-ID');
const selesai=new Date(u.waktuSelesai).toLocaleString('id-ID');
let badge='';
if(u.sudahMengerjakan)badge='<span class="badge badge-done">Selesai</span>';
return '<div class="exam-item"><h3>'+esc(u.judul)+'</h3><p class="meta">'+esc(u.mapel?.namaMapel||'')+' &middot; '+mulai+' &mdash; '+selesai+' &middot; '+u.durasiMenit+' menit '+badge+'</p><div class="actions"><button class="btn btn-primary" onclick="mulaiUjian(\''+u.id+'\')" '+(u.sudahMengerjakan?'disabled':'')+'>Mulai</button></div></div>'
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
document.getElementById('soalNum').textContent=state.currentIdx+1;
document.getElementById('soalTotal').textContent=total;
document.getElementById('progressBar').style.width=((state.currentIdx+1)/total*100)+'%';
const c=document.getElementById('soalContainer');
let html='<div class="question"><div class="num">Soal '+(state.currentIdx+1)+'</div><div class="text">'+esc(s.pertanyaan)+'</div>';
if(s.tipe==='pg'&&s.opsi){
s.opsi.forEach((op,i)=>{
const sel=state.jawaban[s.id]===String(i)?'selected':'';
html+='<div class="option '+sel+'" onclick="jawab(\''+s.id+'','+i+')"><input type="radio" name="soal_'+s.id+'" '+(sel?'checked':'')+'><span><strong>'+String.fromCharCode(65+i)+'</strong>. '+esc(op)+'</span></div>';
});
}else{
html+='<textarea style="width:100%;min-height:120px;padding:10px;border:1px solid var(--border);border-radius:10px;font-size:14px" oninput="jawabTeks(\''+s.id+'',this.value)" placeholder="Tulis jawaban Anda...">'+esc(state.jawaban[s.id]||'')+'</textarea>';
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
document.getElementById('scoreDetail').innerHTML='Benar: <strong>'+d.benar+'</strong> dari <strong>'+d.total+'</strong> poin soal<br>Status: <strong>Selesai</strong>';
hide(document.getElementById('examCard'));show(document.getElementById('resultCard'));
}catch(e){alert(e.message)}
}

// Anti-cheat: report tab switch
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
