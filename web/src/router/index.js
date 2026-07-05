import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import Layout from '../components/Layout.vue'
import DashboardContent from '../views/DashboardContent.vue'
import Nodes from '../views/Nodes.vue'
import NodeDetail from '../views/NodeDetail.vue'
import Sessions from '../views/Sessions.vue'
import AuditLogs from '../views/AuditLogs.vue'
import Terminal from '../views/Terminal.vue'
import SharedTerminal from '../views/SharedTerminal.vue'
import Scripts from '../views/Scripts.vue'

const routes = [
  { path: '/login', name: 'Login', component: Login },
  {
    path: '/',
    component: Layout,
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'Dashboard', component: DashboardContent },
      { path: 'nodes', name: 'Nodes', component: Nodes },
      { path: 'nodes/:id', name: 'NodeDetail', component: NodeDetail },
      { path: 'sessions', name: 'Sessions', component: Sessions },
      { path: 'audit-logs', name: 'AuditLogs', component: AuditLogs },
      { path: 'scripts', name: 'Scripts', component: Scripts },
      { path: 'terminal/:nodeId/:portName?', name: 'Terminal', component: Terminal },
      { path: 'shared-terminal/:nodeId/:sessionId', name: 'SharedTerminal', component: SharedTerminal },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  } else {
    next()
  }
})

export default router
