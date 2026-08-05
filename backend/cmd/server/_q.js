const http = require('http');
const req = (path, token) => new Promise((res, rej) => {
  const opts = { hostname: 'localhost', port: 8080, path: '/api'+path, method: 'GET', headers: {} };
  if (token) opts.headers.Authorization = 'Bearer ' + token;
  http.get(opts, r => { let d=''; r.on('data', c => d+=c); r.on('end', () => res(d)); r.on('error', rej); });
});
const post = (path, body) => new Promise((res, rej) => {
  const data = JSON.stringify(body);
  const opts = { hostname: 'localhost', port: 8080, path: '/api'+path, method: 'POST', headers: { 'Content-Type':'application/json', 'Content-Length': Buffer.byteLength(data) } };
  const r = http.request(opts, rr => { let d=''; rr.on('data', c=>d+=c); rr.on('end',()=>res(d)); rr.on('error',rej); });
  r.write(data); r.end();
});
(async () => {
  const login = JSON.parse(await post('/auth/login', { login:'admin', password:'Admin123' }));
  const at = login.accessToken;
  console.log('=== TahunAjaran ===');
  JSON.parse(await req('/tahun-ajaran', at)).forEach(t => console.log(t.id, t.namaTahunAjaran, 'aktif='+t.isAktif));
  console.log('=== Pokjar ===');
  JSON.parse(await req('/pokjar', at)).forEach(p => console.log(p.id, p.namaPokjar, p.tipe));
  console.log('=== Kelas ===');
  const kelas = JSON.parse(await req('/kelas', at));
  console.log('total='+kelas.length);
  kelas.forEach(k => console.log(' jenjang='+k.jenjang, 'rombel='+k.namaRombel, 'ta='+k.tahunAjaranId, 'pokjar='+k.pokjarId));
  console.log('=== PesertaDidik ===');
  const pd = JSON.parse(await req('/peserta-didik', at));
  console.log('total='+(Array.isArray(pd)?pd.length:'?'));
})().catch(e => { console.error('ERR', e.message); process.exit(1); });
