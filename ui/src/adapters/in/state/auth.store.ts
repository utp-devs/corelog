import { create } from 'zustand'

import { AuthService } from '@/core/auth/application/auth.service'
import type {
  ChangePasswordPayload,
  LoginPayload,
  RegisterPayload,
  UpdateProfilePayload,
} from '@/core/auth/application/ports/auth-repository.port'
import type { User } from '@/core/auth/domain/user'
import { AccountSecurityHttpRepository } from '@/adapters/out/http/account-security-http-repository'
import { AuthHttpRepository } from '@/adapters/out/http/auth-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { tokenStorage } from '@/adapters/out/storage/token-storage'

const authService = new AuthService(new AuthHttpRepository())

type Status = 'idle' | 'loading' | 'authenticated' | 'error'

interface AuthState {
  user: User | null
  status: Status
  error: string | null
  sessionChecked: boolean
  login: (payload: LoginPayload) => Promise<void>
  register: (payload: RegisterPayload) => Promise<void>
  restoreSession: () => Promise<void>
  updateProfile: (payload: UpdateProfilePayload) => Promise<void>
  changePassword: (payload: ChangePasswordPayload) => Promise<void>
  logout: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  status: 'idle',
  error: null,
  sessionChecked: false,

  login: async (payload) => {
    set({ status: 'loading', error: null })
    try {
      const session = await authService.login(payload)
      tokenStorage.set(session.token)
      set({ user: session.user, status: 'authenticated', sessionChecked: true })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'Invalid email or password'), sessionChecked: true })
      throw error
    }
  },

  register: async (payload) => {
    set({ status: 'loading', error: null })
    try {
      const session = await authService.register(payload)
      tokenStorage.set(session.token)
      set({ user: session.user, status: 'authenticated', sessionChecked: true })
    } catch (error) {
      set({ status: 'error', error: extractErrorMessage(error, 'Could not create account'), sessionChecked: true })
      throw error
    }
  },

  restoreSession: async () => {
    const token = tokenStorage.get()
    if (!token) {
      set({ status: 'idle', sessionChecked: true })
      return
    }
    set({ status: 'loading' })
    try {
      const user = await authService.currentUser(token)
      set({ user, status: 'authenticated', sessionChecked: true })
    } catch {
      tokenStorage.clear()
      set({ user: null, status: 'idle', sessionChecked: true })
    }
  },

  updateProfile: async (payload) => {
    const user = await authService.updateProfile(payload)
    set({ user })
  },

  changePassword: async (payload) => {
    await authService.changePassword(payload)
  },

  logout: () => {
    // Cierre en el servidor (best effort): la sesion queda revocada aunque el navegador ya no tenga el token.
    const token = tokenStorage.get()
    if (token) {
      void new AccountSecurityHttpRepository().logout(token).catch(() => undefined)
    }
    tokenStorage.clear()
    set({ user: null, status: 'idle', error: null, sessionChecked: true })
  },
}))
