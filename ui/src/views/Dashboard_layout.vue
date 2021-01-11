<template>
    <div>
		<app-drawer class="app--drawer"></app-drawer>
        <app-toolbar class="app--toolbar"></app-toolbar>

        <div class="card-body" >
            <router-view></router-view>
        </div>

        <v-footer app color="teal darken-2">
            <v-layout row wrap align-center>
            <v-flex xs12>
                <div class="white--text ml-4">
                    <h5>© 2020, <a style="color:#ff9966;font-size:1rem" href="https://www.grycap.upv.es">GRyCAP-I3M-UPV</a>, Universitat Politècnica de Valéncia, Spain.</h5>               
                </div>
            </v-flex>
            </v-layout>
        </v-footer>
    </div>
 
</template>
<script>
import AppDrawer from '@/components/AppDrawer'
import AppToolbar from '@/components/AppToolbar'
export default {
	components: {
		AppDrawer,
		AppToolbar,    
    },
}
</script>
<style scoped>
h5{
    font-size:0.9rem !important
}
</style>