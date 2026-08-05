import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Database,
  FolderOpen,
  Pencil,
  Plus,
  Search,
  Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from './components/ui/alert-dialog'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Card } from './components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './components/ui/dialog'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { PageToolbar } from './components/ui/page'
import { Select } from './components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './components/ui/table'

type Row = Record<string, unknown> & { id: string }

type Field = {
  key: string
  label: string
  type?: 'text' | 'date' | 'select' | 'checkbox'
  options?: { value: string; label: string }[]
  required?: boolean
  placeholder?: string
}

// Column model untuk rendering tabel — digerakkan dari schema, menghilangkan
// cabang `resource === '...'` per sumber daya di header & body.
//   key:           field row, atau kunci virtual berawalan '_' (lihat bawah)
//   render:        cara menampilkan sel
// Special keys:
//   '_updated'  → tanggal updatedAt/createdAt
//   '_periode'  → rentang tanggalMulai–tanggalSelesai + mulai semester genap
//   '_active'   → status aktif tahun ajaran (badge / tombol Set Aktif)
//   '_info'     → teks statis (infoText)
type Column = {
  key: string
  label: string
  render?: 'text' | 'badge' | 'badgeBrand' | 'badgeSelect' | 'date' | 'dateRange' | 'activeStatus' | 'info' | 'boolean'
  primary?: boolean
  mono?: boolean
  truncate?: boolean
  options?: { value: string; label: string }[]
  infoText?: string
}

type Schema = {
  title: string
  description: string
  fields: Field[]
  columns: Column[]
  createDisabled?: boolean
  deleteDisabled?: boolean
  readOnly?: string[]
}

const apiBase = import.meta.env.VITE_API_BASE_URL || '/api'

const schemas: Record<string, Schema> = {
  tutor: {
    title: 'Tutor',
    description: 'Daftar tutor dan pengajar PKBM Tunas Ilmu.',
    fields: [
      { key: 'nama', label: 'Nama Lengkap', required: true, placeholder: 'Nama tutor' },
      {
        key: 'jenisKelamin',
        label: 'Jenis Kelamin',
        type: 'select',
        options: [
          { value: 'L', label: 'L - Laki-laki' },
          { value: 'P', label: 'P - Perempuan' },
        ],
        required: true,
      },
      { key: 'noHp', label: 'No. HP / WhatsApp', required: false, placeholder: '08123456789' },
      { key: 'alamat', label: 'Alamat', required: false, placeholder: 'Alamat lengkap tutor' },
      { key: 'isRppMaker', label: 'Penyusun RPP (berhak upload RPP per jenjang)', type: 'checkbox' },
    ],
    columns: [
      { key: 'nama', label: 'Nama Tutor', primary: true },
      {
        key: 'jenisKelamin',
        label: 'Jenis Kelamin',
        render: 'badgeSelect',
        options: [
          { value: 'L', label: 'L - Laki-laki' },
          { value: 'P', label: 'P - Perempuan' },
        ],
      },
      { key: 'noHp', label: 'No. HP / WA' },
      { key: 'alamat', label: 'Alamat', truncate: true },
      { key: 'isRppMaker', label: 'Penyusun RPP', render: 'boolean' },
    ],
  },
  'orang-tua': {
    title: 'Orang Tua',
    description: 'Daftar orang tua / wali peserta didik.',
    fields: [
      { key: 'namaBapak', label: 'Nama Bapak', required: false, placeholder: 'Nama ayah/bapak' },
      { key: 'nikAyah', label: 'NIK Bapak', required: false, placeholder: 'NIK bapak/ayah (opsional)' },
      { key: 'namaIbu', label: 'Nama Ibu', required: false, placeholder: 'Nama ibu' },
      { key: 'nikIbu', label: 'NIK Ibu', required: false, placeholder: 'NIK ibu (opsional)' },
    ],
    columns: [
      { key: 'namaBapak', label: 'Nama Bapak', primary: true },
      { key: 'nikAyah', label: 'NIK Bapak', mono: true },
      { key: 'namaIbu', label: 'Nama Ibu', primary: true },
      { key: 'nikIbu', label: 'NIK Ibu', mono: true },
      { key: '_info', label: 'Informasi', render: 'info', infoText: 'Wali Peserta Didik' },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
  },
  pokjar: {
    title: 'Pokjar',
    description: 'Daftar kelompok belajar (Pusat / Binaan).',
    fields: [
      { key: 'namaPokjar', label: 'Nama Pokjar', required: true, placeholder: 'Contoh: Pokjar Melati' },
      {
        key: 'tipe',
        label: 'Tipe Pokjar',
        type: 'select',
        options: [
          { value: 'pusat', label: 'Pusat' },
          { value: 'binaan', label: 'Binaan' },
        ],
        required: true,
      },
      { key: 'alamat', label: 'Alamat Pokjar', required: false, placeholder: 'Lokasi/alamat pokjar' },
    ],
    columns: [
      { key: 'namaPokjar', label: 'Nama Pokjar', primary: true },
      {
        key: 'tipe',
        label: 'Tipe Pokjar',
        render: 'badgeSelect',
        options: [
          { value: 'pusat', label: 'Pusat' },
          { value: 'binaan', label: 'Binaan' },
        ],
      },
      { key: 'alamat', label: 'Alamat', truncate: true },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
  },
  'tahun-ajaran': {
    title: 'Tahun Ajaran',
    description: 'Daftar periode tahun akademik.',
    fields: [
      { key: 'namaTahunAjaran', label: 'Tahun Ajaran', required: true, placeholder: 'Contoh: 2025/2026' },
      { key: 'tanggalMulai', label: 'Tanggal Mulai', type: 'date', required: true },
      { key: 'tanggalSelesai', label: 'Tanggal Selesai', type: 'date', required: true },
    ],
    columns: [
      { key: 'namaTahunAjaran', label: 'Tahun Ajaran', primary: true },
      { key: '_periode', label: 'Periode Pelaksanaan', render: 'dateRange' },
      { key: '_active', label: 'Status Keaktifan', render: 'activeStatus' },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
  },
  semester: {
    title: 'Semester',
    description: 'Daftar semester per tahun ajaran. Arsipkan semester yang sudah lewat.',
    createDisabled: true,
    deleteDisabled: true,
    fields: [
      {
        key: 'tahunAjaranId',
        label: 'Tahun Ajaran',
        type: 'select',
        required: true,
      },
      { key: 'namaSemester', label: 'Nama Semester', required: true, placeholder: 'Ganjil / Genap' },
      { key: 'tanggalMulai', label: 'Tanggal Mulai', type: 'date', required: true },
      { key: 'tanggalSelesai', label: 'Tanggal Selesai', type: 'date', required: true },
      { key: 'isArchived', label: 'Diarsipkan', type: 'checkbox' },
    ],
    columns: [
      { key: '_tahunAjaran', label: 'Tahun Ajaran', render: 'text' },
      { key: 'namaSemester', label: 'Semester', primary: true },
      { key: '_periode', label: 'Periode', render: 'dateRange' },
      { key: '_archive', label: 'Status', render: 'archiveStatus' },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
    readOnly: ['namaSemester'],
  },
  mapel: {
    title: 'Mata Pelajaran',
    description: 'Daftar mata pelajaran yang diajarkan.',
    fields: [
      { key: 'namaMapel', label: 'Nama Mata Pelajaran', required: true, placeholder: 'Contoh: Bahasa Indonesia' },
      { key: 'kodeMapel', label: 'Kode Mapel', required: false, placeholder: 'Contoh: BIN-01' },
    ],
    columns: [
      { key: 'namaMapel', label: 'Nama Mata Pelajaran', primary: true },
      { key: 'kodeMapel', label: 'Kode Mapel', render: 'badge', mono: true },
      { key: '_info', label: 'Informasi', render: 'info', infoText: 'Mata Pelajaran LMS' },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
  },
  buku: {
    title: 'Buku Perpustakaan',
    description: 'Master data buku modul yang dipinjamkan kepada peserta didik.',
    fields: [
      { key: 'judul', label: 'Judul Buku', required: true, placeholder: 'Judul buku' },
      { key: 'kodeBuku', label: 'Kode Buku', required: false, placeholder: 'Kode buku (opsional)' },
      { key: 'penerbit', label: 'Penerbit', required: false, placeholder: 'Penerbit (opsional)' },
    ],
    columns: [
      { key: 'judul', label: 'Judul', primary: true },
      { key: 'kodeBuku', label: 'Kode Buku', render: 'badge', mono: true },
      { key: 'penerbit', label: 'Penerbit' },
      { key: '_updated', label: 'Diperbarui', render: 'date' },
    ],
  },
  program: {
    title: 'Program (Paket)',
    description: 'Program PKBM — Paket A, B, C dan jenjang setara.',
    fields: [
      { key: 'kode', label: 'Kode', required: true, placeholder: 'A / B / C' },
      { key: 'nama', label: 'Nama Program', required: true, placeholder: 'Contoh: Paket C' },
      { key: 'jenjangSetara', label: 'Jenjang Setara', required: false, placeholder: 'Contoh: SMA' },
      { key: 'keterangan', label: 'Keterangan', required: false, placeholder: 'Keterangan (opsional)' },
    ],
    columns: [
      { key: 'kode', label: 'Kode', render: 'badgeBrand', mono: true },
      { key: 'nama', label: 'Nama Program', primary: true },
      { key: 'jenjangSetara', label: 'Jenjang Setara' },
      { key: 'keterangan', label: 'Keterangan', truncate: true },
    ],
  },
  fase: {
    title: 'Fase',
    description: 'Fase pembelajaran (A–E) — opsional, untuk kurikulum merdeka.',
    fields: [
      { key: 'kode', label: 'Kode', required: true, placeholder: 'A / B / C / D / E' },
      { key: 'nama', label: 'Nama Fase', required: true, placeholder: 'Contoh: Fase D' },
      { key: 'jenjangSetara', label: 'Jenjang Setara', required: false, placeholder: 'Contoh: SD' },
    ],
    columns: [
      { key: 'kode', label: 'Kode', render: 'badgeBrand', mono: true },
      { key: 'nama', label: 'Nama Fase', primary: true },
      { key: 'jenjangSetara', label: 'Jenjang Setara' },
    ],
  },
}

async function request(path: string, token: string, method = 'GET', body?: unknown) {
  const response = await fetch(apiBase + path, {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: body ? JSON.stringify(body) : undefined,
  })

  const result = response.status === 204 ? null : await response.json().catch(() => ({}))

  if (!response.ok) {
    throw new Error(result?.error || 'Permintaan gagal')
  }
  return result
}

function formatDate(value: unknown): string {
  if (!value) return '-'
  try {
    return new Date(String(value)).toLocaleDateString('id-ID', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    })
  } catch {
    return String(value)
  }
}

function formatDateForInput(value: unknown): string {
  if (!value) return ''
  return String(value).slice(0, 10)
}

function getRowDisplayName(row: Row | null): string {
  if (!row) return ''
  return String(
    row.nama ||
      row.namaPokjar ||
      row.namaTahunAjaran ||
      row.namaMapel ||
      (row.namaBapak ? `Bpk. ${row.namaBapak}` : row.namaIbu ? `Ibu ${row.namaIbu}` : 'Data ini')
  )
}

export function MasterData({
  resource,
  token,
  readOnly,
}: {
  resource: string
  token: string
  readOnly: boolean
}) {
  const schema = schemas[resource] || {
    title: resource,
    description: '',
    fields: [],
    columns: [],
  }

  const [rows, setRows] = useState<Row[]>([])
  const [loading, setLoading] = useState<boolean>(true)
  const [searchQuery, setSearchQuery] = useState<string>('')
  const [currentPage, setCurrentPage] = useState<number>(1)
  const itemsPerPage = 10

  const [isFormOpen, setIsFormOpen] = useState<boolean>(false)
  const [editingRow, setEditingRow] = useState<Row | null>(null)
  const [deletingRow, setDeletingRow] = useState<Row | null>(null)
  const [activatingRow, setActivatingRow] = useState<Row | null>(null)
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false)
  const [taOptions, setTaOptions] = useState<Row[]>([])

  const loadData = () => {
    setLoading(true)
    request('/' + resource, token)
      .then((data) => {
        if (Array.isArray(data)) {
          setRows(data as Row[])
        } else {
          setRows([])
        }
      })
      .catch((e) => {
        toast.error(`Gagal memuat data ${schema.title}: ${String(e.message || e)}`)
        setRows([])
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    setSearchQuery('')
    setCurrentPage(1)
    loadData()
    if (resource === 'semester') {
      request('/tahun-ajaran', token)
        .then((d) => { if (Array.isArray(d)) setTaOptions(d as Row[]) })
        .catch(() => undefined)
    }
  }, [resource, token])

  const filteredRows = useMemo(() => {
    if (!searchQuery.trim()) return rows
    const q = searchQuery.toLowerCase().trim()
    return rows.filter((row) =>
      Object.values(row).some((val) => val !== null && val !== undefined && String(val).toLowerCase().includes(q))
    )
  }, [rows, searchQuery])

  useEffect(() => {
    setCurrentPage(1)
  }, [searchQuery])

  const totalPages = Math.ceil(filteredRows.length / itemsPerPage) || 1
  const paginatedRows = useMemo(() => {
    const start = (currentPage - 1) * itemsPerPage
    return filteredRows.slice(start, start + itemsPerPage)
  }, [filteredRows, currentPage, itemsPerPage])

  // Clamp the page after a delete (or search) shrinks totalPages, otherwise the
  // user can be left on a page past the end with an empty "Belum ada data" view.
  useEffect(() => {
    if (currentPage > totalPages) setCurrentPage(totalPages)
  }, [currentPage, totalPages])

  const handleOpenAdd = () => {
    setEditingRow(null)
    setIsFormOpen(true)
  }

  const handleOpenEdit = (row: Row) => {
    setEditingRow(row)
    setIsFormOpen(true)
  }

  const handleFormSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setIsSubmitting(true)
    const formData = new FormData(event.currentTarget)
    const body: Record<string, unknown> = Object.fromEntries(formData.entries())

    if (resource === 'tahun-ajaran') {
      body.isAktif = editingRow?.isAktif ?? false
    }
    if (resource === 'semester') {
      body.isArchived = formData.has('isArchived')
      body.tanggalMulai = body.tanggalMulai ? String(body.tanggalMulai).slice(0, 10) : undefined
      body.tanggalSelesai = body.tanggalSelesai ? String(body.tanggalSelesai).slice(0, 10) : undefined
    }
    if (resource === 'mapel') {
      body.isActive = editingRow?.isActive ?? true
    }
    if (resource === 'tutor') {
      // checkbox HTML hanya masuk formData saat dicentang → formData.has memberi boolean.
      body.isRppMaker = formData.has('isRppMaker')
    }

    // Validasi sisi klien sebelum mengirim — melengkapi atribut `required` native
    // (yang hanya periksa kosong saat browser memvalidasi). Menangkap kasus di
    // antarmuka yang melewati validasi native maupun validasi format/range.
    for (const f of schema.fields) {
      if (f.required && !String(body[f.key] ?? '').trim()) {
        toast.error(`${f.label} wajib diisi.`)
        setIsSubmitting(false)
        return
      }
    }
    if (resource === 'tahun-ajaran') {
      const mulai = String(body.tanggalMulai || '')
      const selesai = String(body.tanggalSelesai || '')
      if (mulai && selesai && selesai < mulai) {
        toast.error('Tanggal selesai tidak boleh sebelum tanggal mulai.')
        setIsSubmitting(false)
        return
      }
    }
    if (resource === 'orang-tua') {
      for (const k of ['nikAyah', 'nikIbu']) {
        const v = String(body[k] ?? '').trim()
        if (v && !/^\d{16}$/.test(v)) {
          toast.error('NIK harus terdiri dari 16 digit angka.')
          setIsSubmitting(false)
          return
        }
      }
    }

    try {
      await request(
        '/' + resource + (editingRow ? '/' + editingRow.id : ''),
        token,
        editingRow ? 'PUT' : 'POST',
        body
      )
      toast.success(`Data ${schema.title} berhasil ${editingRow ? 'diperbarui' : 'ditambahkan'}`)
      setIsFormOpen(false)
      setEditingRow(null)
      loadData()
    } catch (e: any) {
      toast.error(`Gagal menyimpan: ${String(e.message || e)}`)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleConfirmDelete = async () => {
    if (!deletingRow) return
    setIsSubmitting(true)
    try {
      await request('/' + resource + '/' + deletingRow.id, token, 'DELETE')
      toast.success(`Data ${schema.title} berhasil dihapus`)
      setDeletingRow(null)
      loadData()
    } catch (e: any) {
      toast.error(`Gagal menghapus: ${String(e.message || e)}`)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleConfirmActivate = async () => {
    if (!activatingRow) return
    setIsSubmitting(true)
    try {
      const payload = {
        namaTahunAjaran: activatingRow.namaTahunAjaran,
        tanggalMulai: activatingRow.tanggalMulai,
        tanggalSelesai: activatingRow.tanggalSelesai,
        tanggalMulaiSemesterGenap: activatingRow.tanggalMulaiSemesterGenap || null,
        isAktif: true,
      }
      await request('/tahun-ajaran/' + activatingRow.id, token, 'PUT', payload)
      toast.success(`Tahun ajaran "${activatingRow.namaTahunAjaran}" berhasil diaktifkan`)
      setActivatingRow(null)
      loadData()
    } catch (e: any) {
      toast.error(`Gagal mengaktifkan tahun ajaran: ${String(e.message || e)}`)
    } finally {
      setIsSubmitting(false)
    }
  }

  // Jumlah kolom data (tanpa kolom Aksi) diambil langsung dari schema.columns,
  // dipakai untuk skeleton loading & colspan empty-state agar selaras dengan
  // jumlah <th> sebenarnya.
  const colSpanCount = schema.columns.length + (readOnly ? 0 : 1)

  // Render satu sel tabel dari definisi Column — menggantikan cabang per-resource.
  const renderCell = (col: Column, row: Row) => {
    const cellClass = [
      col.primary ? 'font-medium text-foreground' : 'text-muted-foreground text-sm',
      col.mono ? 'font-mono' : '',
      col.truncate ? 'max-w-xs truncate' : '',
    ].join(' ').trim()

    switch (col.render) {
      case 'badge':
        return (
          <TableCell key={col.key}>
            <Badge variant="outline" className="font-mono text-xs">
              {String(row[col.key] || '-')}
            </Badge>
          </TableCell>
        )
      case 'badgeBrand':
        return (
          <TableCell key={col.key}>
            <Badge variant="brand" className="font-mono text-xs">
              {String(row[col.key] || '-')}
            </Badge>
          </TableCell>
        )
      case 'badgeSelect': {
        const opt = col.options?.find((o) => o.value === row[col.key])
        return (
          <TableCell key={col.key}>
            <Badge variant="outline" className="font-normal text-xs">
              {opt ? opt.label : String(row[col.key] || '-')}
            </Badge>
          </TableCell>
        )
      }
      case 'date':
        return (
          <TableCell key={col.key} className="text-muted-foreground text-xs">
            {col.key === '_updated'
              ? formatDate(row.updatedAt || row.createdAt)
              : formatDate(row[col.key])}
          </TableCell>
        )
      case 'dateRange':
        return (
          <TableCell key={col.key} className="text-muted-foreground text-xs">
            <div>{formatDate(row.tanggalMulai)} — {formatDate(row.tanggalSelesai)}</div>
            {row.tanggalMulaiSemesterGenap ? (
              <div className="text-[10px] text-muted-foreground/80">
                Genap mulai: {formatDate(row.tanggalMulaiSemesterGenap)}
              </div>
            ) : null}
          </TableCell>
        )
      case 'info':
        return (
          <TableCell key={col.key} className="text-muted-foreground text-xs">
            {col.infoText || '-'}
          </TableCell>
        )
      case 'activeStatus':
        return (
          <TableCell key={col.key}>
            {row.isAktif ? (
              <Badge className="bg-success hover:bg-success/90 text-success-foreground font-medium flex items-center gap-1 w-fit text-xs">
                <CheckCircle2 className="h-3 w-3" />
                Aktif
              </Badge>
            ) : !readOnly ? (
              <Button
                size="sm"
                variant="outline"
                className="h-7 text-xs flex items-center gap-1 border-success/40 text-success hover:bg-success/10 hover:text-success"
                onClick={() => setActivatingRow(row)}
              >
                <CheckCircle2 className="h-3 w-3 text-success" />
                Set Aktif
              </Button>
            ) : (
              <Badge variant="outline" className="text-muted-foreground text-xs">
                Nonaktif
              </Badge>
            )}
          </TableCell>
        )
      case 'boolean':
        return (
          <TableCell key={col.key}>
            {row[col.key] ? (
              <Badge className="bg-success hover:bg-success/90 text-success-foreground font-medium flex items-center gap-1 w-fit text-xs">
                <CheckCircle2 className="h-3 w-3" />
                Ya
              </Badge>
            ) : (
              <span className="text-muted-foreground text-xs">—</span>
            )}
          </TableCell>
        )
      case 'archiveStatus':
        return (
          <TableCell key={col.key}>
            {row.isArchived ? (
              <Badge variant="outline" className="text-muted-foreground font-medium text-xs">
                Diarsipkan
              </Badge>
            ) : (
              <Badge className="bg-success hover:bg-success/90 text-success-foreground font-medium flex items-center gap-1 w-fit text-xs">
                <CheckCircle2 className="h-3 w-3" />
                Aktif
              </Badge>
            )}
          </TableCell>
        )
      case 'text':
        if (col.key === '_tahunAjaran') {
          return (
            <TableCell key={col.key} className="text-muted-foreground text-xs">
              {(row.tahunAjaran as Row)?.namaTahunAjaran || String(row.tahunAjaranId || '-')}
            </TableCell>
          )
        }
        return (
          <TableCell key={col.key} className={cellClass}>
            {String(row[col.key] || '-')}
          </TableCell>
        )
      default:
        return (
          <TableCell key={col.key} className={cellClass}>
            {String(row[col.key] || '-')}
          </TableCell>
        )
    }
  }

  return (
    <div className="space-y-4">
      <PageToolbar
        title={`Data Master ${schema.title}`}
        description={schema.description || `${rows.length} data ${schema.title.toLowerCase()} tercatat dalam sistem.`}
        actions={
          !readOnly && !schema.createDisabled && (
            <Button onClick={handleOpenAdd} className="shadow-2xs">
              <Plus className="h-4 w-4 mr-1" />
              Tambah {schema.title}
            </Button>
          )
        }
      />

      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="search"
            placeholder={`Cari data ${schema.title.toLowerCase()}...`}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 h-9 text-sm"
          />
        </div>

        <Badge variant="outline" className="h-9 px-3 py-1 font-normal text-xs flex items-center gap-1.5 self-start sm:self-auto">
          <Database className="h-3.5 w-3.5 text-muted-foreground" />
          {filteredRows.length === rows.length
            ? `${rows.length} Data`
            : `${filteredRows.length} dari ${rows.length} Data`}
        </Badge>
      </div>

      <Card className="rounded-2xl border border-border bg-card shadow-2xs overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="border-b border-border">
              {schema.columns.map((col) => (
                <TableHead key={col.key}>{col.label}</TableHead>
              ))}
              {!readOnly && <TableHead className="text-right">Aksi</TableHead>}
            </TableRow>
          </TableHeader>

          <TableBody>
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  {Array.from({ length: colSpanCount }).map((_, j) => (
                    <TableCell key={j}>
                      <div className="h-4 bg-muted animate-pulse rounded w-24" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : paginatedRows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={colSpanCount} className="h-48 text-center">
                  <div className="flex flex-col items-center justify-center space-y-2 py-6 text-muted-foreground">
                    <FolderOpen className="h-10 w-10 text-muted-foreground/50" />
                    <p className="font-semibold text-foreground text-sm">
                      {searchQuery ? 'Data Tidak Ditemukan' : `Belum ada data ${schema.title}`}
                    </p>
                    <p className="text-xs max-w-sm text-muted-foreground">
                      {searchQuery
                        ? `Tidak ada data yang cocok dengan kata kunci "${searchQuery}". Coba gunakan kata kunci lain.`
                        : `Silakan tambahkan data ${schema.title.toLowerCase()} baru untuk mengisi daftar.`}
                    </p>
                    {!readOnly && !searchQuery && !schema.createDisabled && (
                      <Button size="sm" className="mt-2" onClick={handleOpenAdd}>
                        <Plus className="h-3.5 w-3.5 mr-1" />
                        Tambah {schema.title}
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              paginatedRows.map((row) => (
                <TableRow key={row.id} className="transition-colors">
                  {schema.columns.map((col) => renderCell(col, row))}
                  {!readOnly && (
                    <TableCell>
                      <div className="flex justify-end gap-1.5">
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-8 px-2.5 text-xs"
                          onClick={() => handleOpenEdit(row)}
                        >
                          <Pencil className="h-3.5 w-3.5 mr-1" />
                          Ubah
                        </Button>
                        {!schema.deleteDisabled && (
                          <Button
                            size="sm"
                            variant="destructive"
                            className="h-8 px-2.5 text-xs"
                            onClick={() => setDeletingRow(row)}
                          >
                            <Trash2 className="h-3.5 w-3.5 mr-1" />
                            Hapus
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {filteredRows.length > 0 && (
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3 p-4 border-t bg-muted/20 text-xs">
            <p className="text-muted-foreground">
              Menampilkan {(currentPage - 1) * itemsPerPage + 1} -{' '}
              {Math.min(currentPage * itemsPerPage, filteredRows.length)} dari {filteredRows.length} data
            </p>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                className="h-8 text-xs"
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
              >
                <ChevronLeft className="h-3.5 w-3.5 mr-1" />
                Sebelumnya
              </Button>
              <span className="text-muted-foreground px-2">
                Halaman {currentPage} dari {totalPages}
              </span>
              <Button
                size="sm"
                variant="outline"
                className="h-8 text-xs"
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage === totalPages}
              >
                Selanjutnya
                <ChevronRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            </div>
          </div>
        )}
      </Card>

      {/* Add / Edit Dialog */}
      <Dialog open={isFormOpen} onOpenChange={setIsFormOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editingRow ? 'Ubah' : 'Tambah'} {schema.title}
            </DialogTitle>
            <DialogDescription>
              Isi formulir berikut untuk {editingRow ? 'memperbarui' : 'menambahkan'} data {schema.title.toLowerCase()}.
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleFormSubmit} className="space-y-4 mt-2">
            {schema.fields.map((field) => (
              field.type === 'checkbox' ? (
                <div key={field.key} className="flex items-start gap-2">
                  <input
                    id={field.key}
                    name={field.key}
                    type="checkbox"
                    defaultChecked={!!editingRow?.[field.key]}
                    className="mt-0.5 h-4 w-4 rounded border-gray-300 text-brand-600 focus:ring-brand-500"
                  />
                  <Label htmlFor={field.key} className="text-xs font-medium leading-tight pt-0.5">
                    {field.label}
                  </Label>
                </div>
              ) : (
              <div key={field.key} className="grid gap-2">
                <Label htmlFor={field.key} className="text-xs font-medium">
                  {field.label} {field.required && <span className="text-destructive">*</span>}
                </Label>
                {field.type === 'select' ? (
                  <Select
                    id={field.key}
                    name={field.key}
                    defaultValue={String(editingRow?.[field.key] || field.options?.[0]?.value || '')}
                    required={field.required}
                    disabled={schema.readOnly?.includes(field.key)}
                  >
                    {field.key === 'tahunAjaranId' && resource === 'semester'
                      ? taOptions.map((ta) => (
                          <option key={ta.id} value={ta.id}>
                            {String(ta.namaTahunAjaran || ta.id)}
                          </option>
                        ))
                      : field.options?.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                  </Select>
                ) : field.type === 'date' ? (
                  <Input
                    id={field.key}
                    name={field.key}
                    type="date"
                    defaultValue={formatDateForInput(editingRow?.[field.key])}
                    required={field.required}
                    disabled={schema.readOnly?.includes(field.key)}
                  />
                ) : (
                  <Input
                    id={field.key}
                    name={field.key}
                    type="text"
                    defaultValue={String(editingRow?.[field.key] || '')}
                    placeholder={field.placeholder}
                    required={field.required}
                    disabled={schema.readOnly?.includes(field.key)}
                  />
                )}
              </div>
              )
            ))}

            <DialogFooter className="pt-4 flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setIsFormOpen(false)} disabled={isSubmitting}>
                Batal
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? 'Menyimpan...' : 'Simpan'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation AlertDialog */}
      <AlertDialog open={deletingRow !== null} onOpenChange={(open) => !open && setDeletingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Data {schema.title}?</AlertDialogTitle>
            <AlertDialogDescription>
              Apakah Anda yakin ingin menghapus data &quot;{getRowDisplayName(deletingRow)}&quot;? Tindakan ini tidak dapat dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isSubmitting}>Batal</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={handleConfirmDelete}
              disabled={isSubmitting}
            >
              {isSubmitting ? 'Menghapus...' : 'Hapus'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Tahun Ajaran Active Toggle Confirmation AlertDialog */}
      <AlertDialog open={activatingRow !== null} onOpenChange={(open) => !open && setActivatingRow(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Aktifkan Tahun Ajaran?</AlertDialogTitle>
            <AlertDialogDescription>
              Apakah Anda yakin ingin mengaktifkan Tahun Ajaran &quot;{String(activatingRow?.namaTahunAjaran || '')}&quot;? Tahun ajaran yang sedang aktif saat ini akan otomatis dinonaktifkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isSubmitting}>Batal</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmActivate} disabled={isSubmitting}>
              {isSubmitting ? 'Memproses...' : 'Aktifkan'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
