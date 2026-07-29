import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/Login.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('../views/Layout.vue'),
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
        { path: 'nodes', name: 'nodes', component: () => import('../views/Nodes.vue') },
        { path: 'events', name: 'events', component: () => import('../views/Events.vue') },
        { path: 'rules', name: 'rules', component: () => import('../views/Rules.vue') },
        { path: 'blacklist', name: 'blacklist', component: () => import('../views/Blacklist.vue') },
      ],
    },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!localStorage.getItem('token')) return { name: 'login' }
  return true
})

export default router
