import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '@/stores/auth'

const P = ['PROFESOR', 'ADMIN']
const A = ['ADMIN']
const pendiente = () => import('@/views/comun/EnConstruccion.vue')

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/comun/LoginView.vue'),
    meta: { publica: true }
  },
  {
    path: '/',
    component: () => import('@/layouts/DashboardLayout.vue'),
    children: [
      { path: '', redirect: { name: 'laboratorio' } },

      // Todos
      {
        path: 'laboratorio',
        name: 'laboratorio',
        component: () => import('@/views/alumno/LaboratorioView.vue'),
        meta: { titulo: 'Mi laboratorio', icono: 'mdi-lan', menu: true }
      },
      {
        path: 'perfil',
        name: 'perfil',
        component: () => import('@/views/comun/PerfilView.vue'),
        meta: { titulo: 'Mi perfil', icono: 'mdi-account-circle', menu: true }
      },

      // Maestro y admin
      {
        path: 'profesor/grupos',
        name: 'grupos',
        component: () => import('@/views/profesor/GruposView.vue'),
        meta: { roles: P, titulo: 'Grupos', icono: 'mdi-account-group', menu: true, seccion: 'Docencia' }
      },
      {
        path: 'profesor/plantillas',
        name: 'plantillas',
        component: () => import('@/views/profesor/PlantillasView.vue'),
        meta: { roles: P, titulo: 'Plantillas Ansible', icono: 'mdi-sitemap', menu: true, seccion: 'Docencia' }
      },
      {
        path: 'profesor/ia',
        name: 'ia',
        component: pendiente, // Fase 4
        meta: { roles: P, titulo: 'Asistente IA (Qwen)', icono: 'mdi-robot', menu: true, seccion: 'Docencia' }
      },

      // Solo admin
      {
        path: 'admin/usuarios',
        name: 'usuarios',
        component: () => import('@/views/admin/UsuariosView.vue'),
        meta: { roles: A, titulo: 'Usuarios', icono: 'mdi-account-multiple', menu: true, seccion: 'Administración' }
      },
      {
        path: 'admin/importar',
        name: 'importar',
        component: () => import('@/views/admin/ImportarView.vue'),
        meta: { roles: A, titulo: 'Importar CSV / Excel', icono: 'mdi-file-upload', menu: true, seccion: 'Administración' }
      },
      {
        path: 'admin/conocimiento',
        name: 'conocimiento',
        component: pendiente, // Fase 4
        meta: { roles: A, titulo: 'Base de conocimiento IA', icono: 'mdi-book-open-variant', menu: true, seccion: 'Administración' }
      },
      {
        path: 'admin/monitor',
        name: 'monitor',
        component: () => import('@/views/admin/MonitorView.vue'),
        meta: { roles: A, titulo: 'Workspaces activos', icono: 'mdi-monitor-dashboard', menu: true, seccion: 'Administración' }
      }
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  const auth = useAuth()
  await auth.restaurar()

  if (to.meta.publica) {
    return auth.autenticado ? { name: 'laboratorio' } : true
  }
  if (!auth.autenticado) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.roles && !to.meta.roles.includes(auth.rol)) {
    return { name: 'laboratorio' }
  }
  return true
})

export default router