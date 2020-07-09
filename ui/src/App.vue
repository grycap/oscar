<template>
  <div id="app">
    <template v-if="!$route.meta.public">
      <v-app id="inspire" class="app">
        <v-content>
          <v-container fluid wrap grid-list-md align-start justify-space-between>
            <router-view :minioClient="minioClient" :minio="minio" :openFaaS="openFaaS"/>
          </v-container>
        </v-content>
      </v-app>
    </template>
    <template v-else>
      <transition>
        <keep-alive>
          <router-view></router-view>
        </keep-alive>
      </transition>
    </template>
    <v-snackbar @show-snackbar="onShowSnackbar" center top v-model="snackbar.showBucketContent" :color="snackbar.color" :timeout="snackbar.timeout">
      {{ snackbar.text }}
      <v-btn dark flat @click="snackbar.show = false" icon>
        <v-icon>close</v-icon>
      </v-btn>
    </v-snackbar>
  </div>
</template>

<script>
import AppDrawer from '@/components/AppDrawer'
import AppToolbar from '@/components/AppToolbar'
import AppEvents from './event'
import {Client} from 'minio'
export default {
  name: 'App',
  components: {
    AppDrawer,
    AppToolbar,
    AppEvents
  },
  data: () => ({
    expanded: true,
    rightDrawer: false,
    snackbar: {
      showBucketContent: false,
      auth: '',
      text: '',
      color: '', // ['success', 'info', 'error', 'cyan darken-2']
      timeout: 6000
    },
    breadcrumbList: {},
    minio: {
      endpoint: 'minio-service.minio',
      port: 9000,
      useSSL: false,
      accessKey: 'minio',
      secretKey: 'minio123',
      showSecretKey: false
    },
    openFaaS: {
      endpoint: 'http://oscar-manager.oscar:8080/functions',
      port: null
    },
    minioClient: {}
  }),
  computed: {
  },
  created () {    
    this.auth = localStorage.getItem("authenticated")    
    AppEvents.forEach(item => {
      this.$on(item.name, item.callback)
    })
    window.getApp = this
    window.getApp.$on('APP_SHOW_SNACKBAR', (data) => {
      this.onShowSnackbar(data)
    })
    /**
     * In the case that the minio access configuration is modified, the client instance must be recreated.
     */
    window.getApp.$on('MINIO_RECONNECT', () => {
      this.createMinioClient()
    })

    this.createMinioClient()
  },
  methods: {
    onShowSnackbar (data) {
      this.snackbar.text = data.text
      this.snackbar.color = data.color
      this.snackbar.timeout = data.timeout
      this.snackbar.showBucketContent = true
    },
    createMinioClient () {
      this.minioClient = null
      this.minioClient = new Client({
        endPoint: this.minio.endpoint,
        port: this.minio.port,
        useSSL: this.minio.useSSL,
        accessKey: this.minio.accessKey,
        secretKey: this.minio.secretKey
      })
    }
  },
  watch: {
    '$route' () {
      this.breadcrumbList = this.$route.meta.breadcrumb
    }
  }
}
</script>
