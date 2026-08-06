import { useEffect, useState, type FormEvent } from 'react'
import { Mail } from 'lucide-react'
import { Alert, AlertDescription } from './ui/alert'
import { Button } from './ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { request } from '../lib/api'

interface TutorAccountUser {
  email?: string
}

interface TutorEmailPromptProps {
  token: string
  user: TutorAccountUser
  required: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: (email: string) => void
}

export function TutorEmailPrompt({
  token,
  user,
  required,
  open,
  onOpenChange,
  onSaved,
}: TutorEmailPromptProps) {
  const [email, setEmail] = useState(user.email || '')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setEmail(user.email || '')
    setError('')
  }, [user.email, open])

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      const result = await request('/auth/account', token, 'PUT', { email })
      const savedEmail = String(result?.email || email).trim()
      onSaved(savedEmail)
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        // The first-login prompt cannot be dismissed before the tutor saves
        // an email. It can still be opened and closed from the profile menu
        // after an email has already been saved.
        if (!required || nextOpen) onOpenChange(nextOpen)
      }}
    >
      <DialogContent className={required ? '[&>button]:hidden' : undefined}>
        <DialogHeader>
          <DialogTitle>{required ? 'Lengkapi Pengaturan Akun' : 'Pengaturan Akun Tutor'}</DialogTitle>
          <DialogDescription>
            Masukkan alamat Gmail aktif. Email ini akan tersimpan pada akun tutor
            dan terlihat oleh Administrator.
          </DialogDescription>
        </DialogHeader>
        <form className="grid gap-4" onSubmit={save}>
          <div className="grid gap-2">
            <Label htmlFor="tutor-email">Email Gmail</Label>
            <Input
              id="tutor-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="nama@gmail.com"
              required
              autoComplete="email"
            />
          </div>
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button type="submit" disabled={saving}>
              <Mail className="h-4 w-4" />
              {saving ? 'Menyimpan...' : 'Simpan Email Gmail'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
