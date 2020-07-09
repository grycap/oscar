<template>
  <v-toolbar color="teal darken-1" fixed dark app>
    <v-toolbar-side-icon @click.stop="handleDrawerToggle"></v-toolbar-side-icon>
    <v-btn icon @click.stop="handleDrawerMini">
      <v-icon v-html="mini ? 'chevron_right' : 'chevron_left'"></v-icon>
    </v-btn>
    <v-spacer></v-spacer>
    <v-btn icon @click="handleFullScreen()">
      <v-icon>fullscreen</v-icon>
    </v-btn>
  </v-toolbar>
</template>
<script>
import NotificationList from '@/components/widgets/list/NotificationList'
import Util from '@/util'
/* eslint-disable */
export default {
  name: 'app-toolbar',
  components: {
    NotificationList
  },
  data: () => ({
    mini: false,
    items: [
      {
        icon: 'account_circle',
        href: '#',
        title: 'Profile',
        click: (e) => {
          console.log(e)
        }
      },
      {
        icon: 'settings',
        href: '#',
        title: 'Settings',
        click: (e) => {
          console.log(e)
        }
      },
      {
        icon: 'fullscreen_exit',
        href: '#',
        title: 'Logout',
        click: (e) => {
          window.getApp.$emit('APP_LOGOUT')
        }
      }
    ],
    notificationCounter: 3
  }),
  computed: {
    notificationShow () {
      return (this.notificationCounter > 0)
    }
  },
  methods: {
    handleDrawerToggle () {
      window.getApp.$emit('APP_DRAWER_TOGGLED')      
    },
    handleDrawerMini () {
      this.mini = !this.mini
      window.getApp.$emit('APP_DRAWER_MINI')
    },
    handleFullScreen () {
      Util.toggleFullScreen()
    }
  }
}
</script>
