<template>
    <div class="mb-1">		
        <v-toolbar flat color="white">
            <v-toolbar-title>LOGS: Service {{serviceName}} </v-toolbar-title>
            <v-spacer></v-spacer>
            <v-btn color="primary" dark @click="goBack()">BACK</v-btn>
        </v-toolbar>
        <v-card-title>
            <v-text-field
                v-model="search"
                append-icon="search"
                label="Search"
                single-line
            ></v-text-field>
            <v-spacer></v-spacer>  
            <v-btn flat icon color="blue" @click="handleUpdate()">
                <v-icon>autorenew</v-icon>
            </v-btn>
             <v-btn color="green lighten-2" dark @click="deleteSuccessJobs()">DELETE SUCCESS JOBS</v-btn>          
             <v-btn color="error" dark @click="deleteAllJobs()">DELETE ALL JOBS</v-btn>          
        </v-card-title>
        <v-data-table
            :headers="headers"
            :items="jobs"
            :loading="loading"
            :expand="expand"
            item-key="name"
            :search="search"
            :custom-sort="customSort"
            :must-sort = true
            :pagination.sync="pagination"
            v-show="!show_spinner"
        >
            <template v-slot:items="props">
            <tr >
                <td>{{ props.item.name }}</td>
                <td class="text-xs-center">{{ props.item.status }}</td>
                <td class="text-xs-center">{{ moment(props.item.creation).format("YYYY-MM-DD HH:mm") }}</td>
                <td class="text-xs-center">{{ moment(props.item.start).format("YYYY-MM-DD HH:mm") }}</td>
                <td class="text-xs-center">{{ moment(props.item.finish).format("YYYY-MM-DD HH:mm") }}</td>
                <td class="text-xs-center">
                    <v-icon small class="mr-2" @click="deleteJob(props.item,props.item.name)">delete</v-icon>
                </td>
                <td class="justify-center layout px-0">
                        <v-icon  medium v-show="props.expanded" @click="props.expanded = !props.expanded">expand_less</v-icon>
                        <v-icon  medium v-show="!props.expanded" @click="props.expanded = !props.expanded;moreLogs(props.item.name)">expand_more</v-icon>
                </td>

            </tr>
            </template>
            <template v-slot:expand="props">
            <v-card flat color="black">
                <v-card-text style="font-family: monospace;color:white;white-space: pre-wrap;">{{job_logs}}</v-card-text>
            </v-card>
            </template>
             <template v-slot:no-data>
                <v-alert :value="true" color="error" icon="warning">
                   Sorry, there are no logs to display here :(
                </v-alert>
            </template>
            <v-alert slot="no-results" :value="true" color="error" icon="warning">
                        Your search for "{{ search }}" found no results.
                    </v-alert>
        </v-data-table>
        <div v-show="show_spinner" style="position:fixed; left:50%;">	
            <intersecting-circles-spinner :animation-duration="1200" :size="50" :color="'#0066ff'" />              		
        </div>
    </div>
 
</template>

<script>
import Services from '../components/services';
import { IntersectingCirclesSpinner } from 'epic-spinners'
import moment from 'moment'
/* eslint-disable */
export default {
    mixins:[Services],
    components: {
		IntersectingCirclesSpinner,
	},
	name: 'Logs',
	data () {
    return {
        expand: false,
        serviceName: '',
        show_spinner: true,
        show_alert: false,
        moment : moment,
        search:'',
        loading:true,
        headers: [
            {
            text: 'JOB NAME',
            align: 'start',
            
            value: 'name',
            },
            { text: 'Status',align: 'center', value: 'status' },
            { text: 'Creation Time', align: 'center', value: 'create_time' },
            { text: 'Start time',align: 'center', value: 'start_time' },
            { text: 'Finish Time',align: 'center', value: 'finish_time' },       
            { text: '',align: 'center', value: 'actions' },
            { text: '',align: 'center', value: 'expand' },
        ],
        pagination: {
            descending: true,
            sortBy: 'create_time',
            rowsPerPage: 10
		},
        jobs: [],
        index:'',
        params_delete:'', 
        job_logs: ''
        
    }
  },
	methods: {
        customSort(items, index, isDesc) {
            items.sort((a, b) => {
                if (index === "create_time") {
                    if (!isDesc) {
                        return a.creation - b.creation;
                    } else {
                        return b.creation - a.creation;
                    }
                } else {
                if (!isDesc) {
                    return a[index] < b[index] ? -1 : 1;
                } else {
                    return b[index] < a[index] ? -1 : 1;
                }
                }
            });
            return items;
        },
        handleUpdate(){
             this.listJobsCall(this.serviceName, this.listJobsCallback);
        },
        moreLogs(val){
            var params_logs = {serviceName: this.serviceName, jobName: val}
            this.listJobNameCall(params_logs, this.listJobNameCallback);
        },
        listJobNameCallback(response){
                this.job_logs = response  //remember to handle error
        },
        goBack(){
            this.$router.push({name: "Functions"})
        },
        deleteJob(job,job_name){
            this.index = this.jobs.indexOf(job)
            this.params_delete = {serviceName: this.serviceName, jobName: job_name}
            if (confirm('Are you sure you want to delete this job?')) {
                this.deleteJobCall(this.params_delete, this.deleteJobCallback);
			}
        },
        deleteJobCallback(response){
            if (response.status==204 ) {   //check response
                this.jobs.splice(this.index, 1)
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `Job ${this.params_delete.jobName} was deleted`, color: 'success' })           
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
            }
        },
        deleteAllJobs(){
            var params = {
                serviceName: this.serviceName,
                all:true
            }
            if (confirm('Are you sure you want to delete this jobs?')) {
                this.deleteAllJobCall(params, this.deleteAllJobCallback);
			}

        },
        deleteAllJobCallback(response){
            if (response.status==204) {   //check response
                window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `All Jobs had been deleted`, color: 'success' })    
                this.jobs = []
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
			}
            
        },
        deleteSuccessJobs(){
            var params = {
                serviceName: this.serviceName,
                all:false
            }
            if (confirm('Are you sure you want to delete the successful jobs?')) {
                this.deleteAllJobCall(params, this.deleteSuccessJobCallback);
			}

        },
        deleteSuccessJobCallback(response){
            var _this = this
            if (response.status==204) {   //check response
                setTimeout(function(){
                    _this.handleUpdate()
                },3000)
                window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `Successful Jobs had been deleted`, color: 'success' })    
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
			}
            
        },
        listJobsCallback(response){
            if(Object.keys(response).length > 0){
                this.show_spinner = false;
				this.jobs =  Object.keys(response).map((key,index) => {
					return {
						name: key,
						status: response[key].status,
						creation: Date.parse(response[key].creation_time),
						start: response[key].start_time,
						finish: response[key].finish_time,
					}
				})
				this.loading = false;
            }else{
                this.show_spinner=false
                this.loading = false
                this.jobs = []
            }
        },
		
	},
	created: function () {
         if(this.$route.params.serviceName){
             this.serviceName = this.$route.params.serviceName
                this.listJobsCall(this.serviceName, this.listJobsCallback);
        }
  	}	
}
</script>

<style scoped>
  h1, h2 {
    font-weight: normal;
  }
  ul {
    list-style-type: none;
    padding: 0;
  }
  li {
    display: inline-block;
    margin: 0 10px;
  }
  a {
    color: #42b983;
  }
  .openFaas {
    background-color: #e6e7e8 !important;
  }

  .clickable {
  cursor: pointer;
}
</style>
