import Vue from 'vue'
import './plugins/vuetify'
import App from './App.vue'
import router from './router/index'
import './registerServiceWorker'
// import VueMaterial from 'vue-material'
// import 'vue-material/dist/vue-material.min.css'

// Vue.use(VueMaterial)
Vue.config.productionTip = false

window.axios = require('axios');
window.axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

new Vue({
  router,
  render: h => h(App)
}).$mount('#app')
