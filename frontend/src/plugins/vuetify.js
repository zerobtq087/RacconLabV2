import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import { es } from 'vuetify/locale'

const raccoonDark = {
  dark: true,
  colors: {
    background: '#0f1115',
    surface: '#171a21',
    'surface-variant': '#232733',
    primary: '#f97316',
    secondary: '#38bdf8',
    success: '#22c55e',
    warning: '#eab308',
    error: '#ef4444',
    info: '#60a5fa'
  }
}

const raccoonLight = {
  dark: false,
  colors: {
    background: '#f5f6f8',
    surface: '#ffffff',
    primary: '#ea580c',
    secondary: '#0284c7',
    success: '#16a34a',
    warning: '#ca8a04',
    error: '#dc2626',
    info: '#2563eb'
  }
}

export default createVuetify({
  locale: { locale: 'es', messages: { es } },
  theme: {
    defaultTheme: localStorage.getItem('raccoon_theme') || 'raccoonDark',
    themes: { raccoonDark, raccoonLight }
  },
  defaults: {
    VCard: { rounded: 'lg', border: true, elevation: 0 },
    VBtn: { rounded: 'lg', class: 'text-none font-weight-bold' },
    VTextField: { variant: 'outlined', density: 'comfortable', color: 'primary' },
    VSelect: { variant: 'outlined', density: 'comfortable', color: 'primary' },
    VTextarea: { variant: 'outlined', color: 'primary' }
  }
})