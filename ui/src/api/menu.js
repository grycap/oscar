const Menu = [
  {header: 'Apps'},
  {
    title: 'Dashboard',
    group: 'apps',
    icon: 'dashboard',
    name: 'Dashboard'
  },
  {
    title: 'Storage',
    group: 'apps',
    icon: 'cloud',
    name: 'Storage',
    active: false,
    items: [
      // { name: 'post', title: 'Post', component: 'components/widget-post' },
    ]
  },
  {
    title: 'Functions',
    group: 'apps',
    icon: 'functions',
    name: 'Functions'
  },
  {
    title: 'Settings',
    group: 'apps',
    icon: 'settings',
    name: 'Settings'
 },
  {
    title: 'Log Out',
    group: 'apps',
    icon: 'exit_to_app',
    name: 'Login'
 }
]
// reorder menu
Menu.forEach((item) => {
  if (item.items) {
    item.items.sort((x, y) => {
      let textA = x.title.toUpperCase()
      let textB = y.title.toUpperCase()
      return (textA < textB) ? -1 : (textA > textB) ? 1 : 0
    })
  }
})

export default Menu
