import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { nextTick } from 'vue'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import AuthView from './AuthView.vue'
import { useAuthStore } from '../../stores/auth'

const replace = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/register', query: {} }),
  useRouter: () => ({ replace }),
}))

describe('registration form feature', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    replace.mockReset()
    localStorage.clear()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('fills the real registration form, submits it, stores the session and redirects to projects', async () => {
    const fetchMock = vi.mocked(fetch)
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({
      user: {
        id: '11111111-1111-1111-1111-111111111111',
        email: 'artist@example.com',
        name: 'Test Artist',
      },
      token: 'test-session-token',
      expires_at: '2026-10-07T12:00:00Z',
    }), {
      status: 201,
      headers: { 'Content-Type': 'application/json' },
    }))

    const wrapper = mount(AuthView, {
      global: {
        plugins: [ElementPlus],
      },
    })

    const inputs = wrapper.findAll('input')
    expect(inputs).toHaveLength(4)

    await inputs[0].setValue('artist@example.com')
    await inputs[1].setValue('Test Artist')
    await inputs[2].setValue('StrongPassword123!')
    await inputs[3].setValue('StrongPassword123!')

    const form = wrapper.find('form')
    expect(form.exists()).toBe(true)
    await form.trigger('submit')
    await nextTick()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/auth/register')
    expect(init?.method).toBe('POST')
    expect(init?.headers).toMatchObject({
      Accept: 'application/json',
      'Content-Type': 'application/json',
    })
    expect(JSON.parse(String(init?.body))).toEqual({
      email: 'artist@example.com',
      name: 'Test Artist',
      password: 'StrongPassword123!',
    })

    const auth = useAuthStore()
    expect(auth.user).toEqual({
      id: '11111111-1111-1111-1111-111111111111',
      email: 'artist@example.com',
      name: 'Test Artist',
    })
    expect(auth.isAuthenticated).toBe(true)
    expect(wrapper.text()).not.toContain('Authentication failed')
    expect(replace).toHaveBeenCalledWith('/projects')
  })

  it('does not send the form when passwords differ', async () => {
    const fetchMock = vi.mocked(fetch)
    const wrapper = mount(AuthView, {
      global: {
        plugins: [ElementPlus],
      },
    })

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('artist@example.com')
    await inputs[1].setValue('Test Artist')
    await inputs[2].setValue('StrongPassword123!')
    await inputs[3].setValue('DifferentPassword123!')

    await wrapper.find('form').trigger('submit')
    await nextTick()

    expect(fetchMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Passwords do not match')
  })
})
