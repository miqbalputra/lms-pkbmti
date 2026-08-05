import { useEffect, useRef, type PointerEvent } from 'react'
import { Trash2 } from 'lucide-react'
import { Button } from './button'
import { Label } from './label'

// Signature — a hand-rolled signature pad (canvas + pointer events). Emits a
// data:image/png;base64 string via onChange on every stroke. Reused by the
// Presensi module and the Peminjaman/Pengembalian Buku forms (PRD §1).
export function Signature({
  value,
  onChange,
  userName,
  label = 'Tanda Tangan Pengajar / Wali Kelas',
}: {
  value: string
  onChange: (v: string) => void
  userName: string
  label?: string
}) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  const strokeColor = useRef('#1d2939')
  // Tracks the last value we emitted ourselves (from drawing/clearing) so the
  // external-value effect does not repaint the canvas mid-stroke and clobber
  // the user's active drawing. Only externally-driven value changes (e.g.
  // loading a saved signature) trigger a repaint.
  const lastEmitted = useRef('')

  // Repaint the canvas when value changes from the outside (e.g. loading a
  // saved record). Skip when the change originated from our own draw/clear.
  useEffect(() => {
    if (value === lastEmitted.current) return
    lastEmitted.current = value
    const c = canvas.current
    if (!c) return
    const ctx = c.getContext('2d')!
    ctx.clearRect(0, 0, c.width, c.height)
    if (!value) return
    const img = new Image()
    img.onload = () => {
      if (canvas.current === c) ctx.drawImage(img, 0, 0, c.width, c.height)
    }
    img.src = value
  }, [value])

  function point(e: PointerEvent<HTMLCanvasElement>) {
    const c = canvas.current!
    const r = c.getBoundingClientRect()
    return {
      x: ((e.clientX - r.left) * c.width) / r.width,
      y: ((e.clientY - r.top) * c.height) / r.height,
    }
  }

  function start(e: PointerEvent<HTMLCanvasElement>) {
    drawing.current = true
    const c = canvas.current!
    const p = point(e)
    const ctx = c.getContext('2d')!
    strokeColor.current = getComputedStyle(c).color || '#1d2939'
    ctx.beginPath()
    ctx.moveTo(p.x, p.y)
    c.setPointerCapture(e.pointerId)
  }

  function draw(e: PointerEvent<HTMLCanvasElement>) {
    if (!drawing.current) return
    const c = canvas.current!
    const p = point(e)
    const ctx = c.getContext('2d')!
    ctx.lineWidth = 2.5
    ctx.lineCap = 'round'
    ctx.strokeStyle = strokeColor.current
    ctx.lineTo(p.x, p.y)
    ctx.stroke()
    const data = c.toDataURL('image/png')
    lastEmitted.current = data
    onChange(data)
  }

  function clear() {
    const c = canvas.current!
    c.getContext('2d')!.clearRect(0, 0, c.width, c.height)
    lastEmitted.current = ''
    onChange('')
  }

  return (
    <div className="rounded-2xl border border-border bg-secondary/30 p-5 space-y-3">
      <div className="flex items-center justify-between">
        <Label className="font-bold text-sm text-foreground">{label}</Label>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={clear}
          className="h-8 text-xs text-destructive hover:text-destructive/80"
        >
          <Trash2 className="h-3.5 w-3.5 mr-1" /> Hapus Tanda Tangan
        </Button>
      </div>

      <canvas
        className="h-40 w-full touch-none rounded-xl border border-dashed border-border bg-card shadow-2xs cursor-crosshair text-foreground"
        ref={canvas}
        width="700"
        height="200"
        onPointerDown={start}
        onPointerMove={draw}
        onPointerUp={() => (drawing.current = false)}
      />

      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs border-t border-border/80 pt-2.5">
        <span className="text-muted-foreground font-medium">
          {value ? '✅ Tanda tangan siap disimpan.' : '✍️ Goreskan tanda tangan jari/mouse pada kotak di atas.'}
        </span>
        <div className="flex items-center gap-1.5 text-foreground font-bold">
          <span className="text-muted-foreground font-normal">Nama Pengajar:</span>
          <span className="text-primary font-extrabold uppercase tracking-wider bg-primary/10 px-2 py-0.5 rounded-md">
            {userName}
          </span>
        </div>
      </div>
    </div>
  )
}