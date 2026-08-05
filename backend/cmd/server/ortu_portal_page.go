package main

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) serveOrangTuaPortalPage(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	return c.SendString(ortuPortalHTML)
}

var ortuPortalHTML = `<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Portal Orang Tua — PKBM Tunas Ilmu</title>
<style>
:root{--brand:#1c5740;--gold:#d4af37;--bg:#f5f7f6;--card:#fff;--border:#e5e7eb;--text:#222;--muted:#666;--success:#16a34a;--danger:#dc2626}
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
.btn-outline{background:#fff;border:1px solid var(--border);color:var(--text)}.btn-outline:hover{background:#f9fafb}
.btn:disabled{opacity:.5;cursor:not-allowed}
.error{background:#fef2f2;border:1px solid #fecaca;color:var(--danger);padding:10px 14px;border-radius:10px;font-size:13px;margin-bottom:12px;display:none}
.error.show{display:block}
.child-card{border:1px solid var(--border);border-radius:12px;padding:16px;margin-bottom:12px;cursor:pointer;transition:all .15s}
.child-card:hover{border-color:var(--brand);box-shadow:0 2px 8px rgba(0,0,0,.08)}
.child-card h3{font-size:16px;margin:0 0 4px}
.child-card .meta{font-size:13px;color:var(--muted);margin:0}
.table{width:100%;border-collapse:collapse;font-size:13px;margin-top:12px}
.table th,.table td{padding:8px 12px;border-bottom:1px solid var(--border);text-align:left}
.table th{font-weight:600;color:var(--muted);font-size:12px;text-transform:uppercase}
.badge{display:inline-block;padding:2px 8px;border-radius:6px;font-size:11px;font-weight:600}
.badge-hadir{background:#dcfce7;color:var(--success)}
.badge-alpha{background:#fef2f2;color:var(--danger)}
.badge-terlambat{background:#fef9c3;color:#a16207}
.hidden{display:none}
.nav{display:flex;gap:8px;margin-bottom:16px}
.nav-item{padding:8px 16px;border-radius:8px;font-size:13px;font-weight:600;cursor:pointer;border:1px solid var(--border);background:#fff;transition:all .15s}
.nav-item.active{background:var(--brand);color:#fff;border-color:var(--brand)}
@media(max-width:640px){
  .wrap{padding:8px}
  .pad{padding:16px}
  h1{font-size:18px}
  .table{font-size:12px}
  .table th,.table td{padding:6px 8px}
  .nav{flex-wrap:wrap}
  .nav-item{padding:6px 12px;font-size:12px}
}
</style>
</head>
<body>
<div class="wrap">
<!-- Login -->
<div id="loginCard" class="card">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Portal Orang Tua</div></div>
<div class="gold"></div>
<div class="pad">
<h1>Masuk Portal Orang Tua</h1>
<p style="color:var(--muted);font-size:13px;margin-bottom:16px">Masukkan NIK Anda dan NISN anak untuk melihat data akademik.</p>
<div id="loginError" class="error"></div>
<div id="loginForm">
<label>NIK Orang Tua</label>
<input type="text" id="nik" placeholder="Masukkan 16 digit NIK" maxlength="20" autocomplete="off">
<label>NISN Anak</label>
<input type="text" id="nisn" placeholder="Masukkan NISN anak" maxlength="20" autocomplete="off">
<button class="btn btn-primary" onclick="doLogin()" id="loginBtn">Masuk</button>
</div>
</div>
</div>

<!-- Portal -->
<div id="portalCard" class="card hidden">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name">Portal Orang Tua</div></div>
<div class="gold"></div>
<div class="pad">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
<h1 style="margin:0">Data Anak</h1>
<button class="btn btn-outline" onclick="doLogout()" style="font-size:12px;padding:6px 12px">Keluar</button>
</div>
<div id="childrenList"></div>
</div>
</div>

<!-- Child Detail -->
<div id="detailCard" class="card hidden">
<div class="head"><div class="org">PKBM Tunas Ilmu</div><div class="name" id="detailName">Detail Anak</div></div>
<div class="gold"></div>
<div class="pad">
<button class="btn btn-outline" onclick="showPortal()" style="margin-bottom:12px;font-size:12px;padding:6px 12px">&larr; Kembali</button>
<div class="nav">
<div class="nav-item active" onclick="showTab('nilai',this)">Nilai</div>
<div class="nav-item" onclick="showTab('presensi',this)">Presensi</div>
<div class="nav-item" onclick="showTab('rapor',this)">Rapor</div>
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
const nik=document.getElementById('nik').value.trim();
const nisn=document.getElementById('nisn').value.trim();
if(!nik||!nisn){showError('loginError','NIK dan NISN wajib diisi');return}
document.getElementById('loginError').classList.remove('show');
document.getElementById('loginBtn').disabled=true;document.getElementById('loginBtn').textContent='Masuk...';
try{
const r=await fetch(API+'/orang-tua/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({nik,nisn})});
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
if(!state.anakList.length){c.innerHTML='<p style="color:var(--muted)">Tidak ada data anak ditemukan.</p>';return}
c.innerHTML=state.anakList.map(a=>'<div class="child-card" onclick="openAnak(\''+a.id+'\')"><h3>'+esc(a.nama)+'</h3><p class="meta">NISN: '+esc(a.nisn)+' &middot; Kelas '+String(a.kelas?.jenjang||'')+esc(a.kelas?.namaRombel||'')+'</p></div>').join('');
}

async function openAnak(id){
state.anakId=id;
const anak=state.anakList.find(a=>a.id===id);
document.getElementById('detailName').textContent=anak?anak.nama:'Detail Anak';
hide(document.getElementById('portalCard'));show(document.getElementById('detailCard'));
showTab('nilai',document.querySelector('.nav-item'));
}

async function showTab(tab,el){
document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));
if(el)el.classList.add('active');
const c=document.getElementById('tabContent');
c.innerHTML='<p style="color:var(--muted)">Memuat...</p>';
try{
if(tab==='nilai'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/nilai',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
if(!d.length){c.innerHTML='<p style="color:var(--muted)">Belum ada data nilai.</p>';return}
c.innerHTML='<table class="table"><thead><tr><th>Mata Pelajaran</th><th>NP</th><th>NK</th><th>Predikat NP</th><th>Predikat NK</th></tr></thead><tbody>'+d.map(r=>'<tr><td>'+esc(r.mapel?.namaMapel||'-')+'</td><td>'+(r.npAkhir!=null?Number(r.npAkhir).toFixed(1):'-')+'</td><td>'+(r.nkAkhir!=null?Number(r.nkAkhir).toFixed(1):'-')+'</td><td>'+(r.predikatNP||'-')+'</td><td>'+(r.predikatNK||'-')+'</td></tr>').join('')+'</tbody></table>';
}else if(tab==='presensi'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/presensi',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
if(!d.length){c.innerHTML='<p style="color:var(--muted)">Belum ada data presensi.</p>';return}
c.innerHTML='<table class="table"><thead><tr><th>Tanggal</th><th>Status</th><th>Catatan</th></tr></thead><tbody>'+d.map(r=>{const cls=r.statusKehadiran==='Hadir'?'badge-hadir':r.statusKehadiran==='Alfa'?'badge-alpha':'badge-terlambat';return'<tr><td>'+String(r.presensi?.tanggal||'').slice(0,10)+'</td><td><span class="badge '+cls+'">'+esc(r.statusKehadiran)+'</span></td><td>'+esc(r.catatan||'')+'</td></tr>'}).join('')+'</tbody></table>';
}else if(tab==='rapor'){
const r=await fetch(API+'/orang-tua/anak/'+state.anakId+'/rapor',{headers:{Authorization:'Bearer '+state.token}});
const d=await r.json();if(!r.ok)throw new Error(d.error);
let html='<h2 style="margin-bottom:8px">Rekap Nilai</h2>';
if(d.rekapNilai&&d.rekapNilai.length){
html+='<table class="table"><thead><tr><th>Mata Pelajaran</th><th>NP Akhir</th><th>NK Akhir</th><th>NA Akhir</th></tr></thead><tbody>'+d.rekapNilai.map(r=>'<tr><td>'+esc(r.mapel?.namaMapel||'-')+'</td><td>'+(r.npAkhir!=null?Number(r.npAkhir).toFixed(1):'-')+'</td><td>'+(r.nkAkhir!=null?Number(r.nkAkhir).toFixed(1):'-')+'</td><td>'+(r.naAkhir!=null?Number(r.naAkhir).toFixed(1):'-')+'</td></tr>').join('')+'</tbody></table>';
}else{html+='<p style="color:var(--muted)">Belum ada data rekap.</p>'}
if(d.catatanRapor&&d.catatanRapor.catatanWali){
html+='<h2 style="margin-top:16px;margin-bottom:8px">Catatan Wali Kelas</h2>';
html+='<div style="background:var(--bg);padding:12px;border-radius:8px;font-size:14px;line-height:1.6">'+esc(d.catatanRapor.catatanWali)+'</div>';
if(d.catatanRapor.naikKelas!=null){
html+='<p style="margin-top:8px;font-weight:600">Keputusan: '+(d.catatanRapor.naikKelas?'Naik Kelas':'Tidak Naik Kelas')+'</p>';
if(d.catatanRapor.kenaikanKe)html+='<p style="font-size:13px;color:var(--muted)">Kenaikan ke: '+esc(d.catatanRapor.kenaikanKe)+'</p>';
}
}
c.innerHTML=html;
}
}catch(e){c.innerHTML='<p style="color:var(--danger)">'+esc(e.message)+'</p>'}
}

function doLogout(){state.token='';showLogin()}
function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}
</script>
</body>
</html>`
