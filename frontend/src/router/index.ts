/**
 * router/index.ts
 *
 * Manual routes for ./src/pages/*.vue
 */

// Composables
import { createRouter, createWebHistory } from 'vue-router'
import Index from '@/pages/index.vue'
import Login from '@/pages/Login.vue'
import Importar from '@/pages/Importar.vue'
import Usuarios from '@/pages/Usuarios.vue'
import Grupos from '@/pages/Grupos.vue'
import Laboratorio from '@/pages/Laboratorio.vue'
import Monitor from '@/pages/Monitor.vue'
import Plantillas from '@/pages/Plantillas.vue'
import Settings from '@/pages/Settings.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: Index,
      meta: { requiresAuth: true },
    },
    {
      path: '/usuarios',
      component: Usuarios,
      meta: { requiresAuth: true, roles: ['ADMIN'] },
    },
    {
      path: '/importar',
      component: Importar,
      meta: { requiresAuth: true, roles: ['ADMIN'] },
    },
    {
      path: '/grupos',
      component: Grupos,
      meta: { requiresAuth: true, roles: ['ADMIN', 'PROFESOR'] },
    },
    {
      path: '/laboratorio',
      component: Laboratorio,
      meta: { requiresAuth: true, roles: ['ALUMNO', 'PROFESOR'] },
    },
    {
      path: '/plantillas',
      component: Plantillas,
      meta: { requiresAuth: true, roles: ['PROFESOR'] },
    },
    {
      path: '/monitor',
      component: Monitor,
      meta: { requiresAuth: true, roles: ['ADMIN'] },
    },
    {
      path: '/settings',
      component: Settings,
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      component: Login,
      meta: { requiresAuth: false, requiresGuest: true },
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  // Si no tenemos usuario, lo pedimos al backend (la sesión vive en la cookie)
  if (authStore.userAcc.id == '' && (to.meta.requiresAuth || to.meta.requiresGuest)) {
    await authStore.fetchProfile()
  }

  // Páginas que solo puede ver cierto rol: meta: { roles: ['ADMIN'] }
  const roles = to.meta.roles as string[] | undefined

  if (to.meta.requiresAuth && authStore.userAcc.id == '') {
    next('/login')
  } else if (to.meta.requiresGuest && authStore.userAcc.id != '') {
    next('/')
  } else if (roles && !roles.includes(authStore.userAcc.rol)) {
    next('/')
  } else {
    next()
  }
})

export default router
