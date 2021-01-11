import Vue from 'vue'
import Router from 'vue-router'
import paths from './paths'

Vue.use(Router)

const router = new Router({
  base: '/',
  // mode: 'history',
  linkActiveClass: 'active',
  routes: paths
})
// router gards
router.beforeEach((to, from, next) => {
  if (to.meta.public) {
     next()
  } else {
    var auth = localStorage.getItem("authenticated");    
    if(typeof auth != 'undefined' && auth == "true"){      
      next()
    }
    else {
      next('/login')
    }
  }
})

router.afterEach((to, from) => {
  // ...
})

export default router
