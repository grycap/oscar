<template> 
    <v-layout row wrap class="mb-1">      
    <v-flex xs12>
        <v-card id="functions">
			<v-card-title>
				<v-text-field
					v-model="search"
					append-icon="search"
					label="Search"
					single-line
				></v-text-field>
				<v-spacer></v-spacer>            
				<FunctionForm :openFaaS="openFaaS" @SHOWSPINNER="handleSHOW()"></FunctionForm>
			</v-card-title>
			
			<v-data-table 
				:headers="headers"
				:items="services"
				:loading="loading"
				class="elevation-1"
				item-key="service"
				:search="search"
				:expand="expand"
				v-show="!show_spinner"
			>
				<template v-slot:items="props">
				<tr >
					<td class="text-xs-center">{{ props.item.service }}</td>
					<td class="text-xs-center">{{ props.item.container }}</td>
					<td class="text-xs-center">{{ props.item.cpu }}</td>
					<td class="text-xs-center">{{ props.item.memory }}</td>
					<td class="justify-center layout px-0">
						<v-icon small class="mr-2" @click="editFunction(props.item)">edit</v-icon>
						<v-icon small class="mr-2" @click="deleteFunction(props.item,props.item.service)">delete</v-icon>
					</td>
					<td class="text-xs-center">
						<v-btn small outline color="indigo" @click="goLogs(props.item.service)">LOGS</v-btn>
					</td>
					<td class="text-xs-center">
						<v-icon  medium v-show="props.expanded" @click="props.expanded = !props.expanded">expand_less</v-icon>
						<v-icon  medium v-show="!props.expanded" @click="props.expanded = !props.expanded;showEnvVars(props.item.envVars.Variables)">expand_more</v-icon>
					</td>
				</tr>
				</template>

				<template v-slot:expand="props" style="margin-top:1rem">
					 
					<div class=" div-list-content" >
						<v-card style="padding:0 2rem 2rem 2rem;">

							<v-tabs
								v-model="model"
								centered
								slider-color="yellow"
							>
								<v-tab
								class="text-xs-center"
								:href="`#tab-service`"
								>
									Service Info
								</v-tab>
								
							</v-tabs>

							<v-tabs-items v-model="model" >
								<v-tab-item  :value="`tab-service`">
									<v-card flat>
										<v-card-text class="custom-padding xs6"> <strong>Name: </strong> {{props.item.service}}</v-card-text>
										<v-card-text class="custom-padding"><strong>Image: </strong> {{props.item.container}}</v-card-text>
										<v-card-text class="custom-padding"><strong>Environment variables: </strong> 
											<pre v-show="Object.keys(props.item.envVars.Variables).length!==0" id="json-renderer"></pre>
										</v-card-text>
										
										<v-card-actions>
											<span class="custom-padding" style="padding:10px;"><strong>Inputs:</strong></span>
										</v-card-actions>
								
										<div class="row" style="margin:15px 30px 0px 30px;">
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Path</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Storage Provider</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Prefix</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Suffix</b>
											</div>
										</div>
										<div v-for="(val, i) in props.item.inputs" :key="'A'+ i"  class="row" style="margin:10px 30px 20px 30px;border-bottom:1px solid #eee;padding-bottom:10px;">
											<div class="col-3 col-md-3 text-left">
												<span class="d-inline d-md-none">{{val.path}}</span>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<span class="d-inline d-md-none">{{val.storage_provider}}</span>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<div v-for="(val,i) in val.prefix" :key="'B'+ i">
													<span class="d-inline d-md-none">{{val}}</span>
												</div>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<div v-for="(val,i) in val.suffix" :key="'C'+ i">
													<span class="d-inline d-md-none">{{val}}</span>
												</div>
											</div> 
										</div>
										
										<v-card-actions>
											<span class="custom-padding" style="padding:10px;"><strong>Outputs:</strong></span>
										</v-card-actions>
										<div class="row" style="margin:15px 30px 0px 30px;">
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Path</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Storage Provider</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Prefix</b>
											</div>
											<div class="col-3 col-md-3 text-left d-md-inline" style="background-color:#eee;">
												<b>Suffix</b>
											</div>
										</div>
										<div v-for="(val, i) in props.item.outputs" :key="'D'+ i"  class="row" style="margin:10px 30px 20px 30px;border-bottom:1px solid #eee;padding-bottom:10px;">
											<div class="col-3 col-md-3 text-left">
												<span class="d-inline d-md-none">{{val.path}}</span>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<span class="d-inline d-md-none">{{val.storage_provider}}</span>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<div v-for="(val,i) in val.prefix" :key="'E'+ i">
													<span class="d-inline d-md-none">{{val}}</span>
												</div>
											</div> 
											<div class="col-3 col-md-3 text-left">
												<div v-for="(val,i) in val.suffix" :key="'F'+ i">
													<span class="d-inline d-md-none">{{val}}</span>
												</div>
											</div> 
										</div>
									</v-card>
								</v-tab-item>
								<v-tab-item :value="`tab-storage`">
									<v-card flat>
										<v-card-text class="custom-padding xs6"> <strong>S3: </strong></v-card-text>
										<div class="row container">
											<input type="password" class="form-control" id="access_key" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="Access Key" style="margin-bottom:5px;">
											<input type="password" class="form-control" id="secret" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="Secret Key" style="margin-bottom:5px;">
											<input type="text" class="form-control" id="region_key" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="Region" style="margin-bottom:5px;">

										</div>

										<v-card-text class="custom-padding xs6"> <strong>ONEDATA: </strong></v-card-text>
										<div class="row container">
											<input type="text" class="form-control" id="access_key" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="ONE PROVIDER HOST" style="margin-bottom:5px;">
											<input type="password" class="form-control" id="secret" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="TOKEN" style="margin-bottom:5px;">
											<input type="text" class="form-control" id="region_key" aria-describedby="emailHelp" :disabled="disable_storage" placeholder="SPACE" style="margin-bottom:5px;">

										</div>

										<div class="row col-12" style="margin-top:5rem;justify-content: flex-end;">
											<v-btn
												color="primary"
												@click="edit_storage()"
											>
												EDIT
											</v-btn>

											<v-btn
												color="error"
												@click="cancel_storage()"
												:disabled="disable_storage"
											>
												CANCEL
											</v-btn>
											
											<v-btn
												color="success"
												@click="done_storage()"
												:disabled="disable_storage"
											>
												DONE
											</v-btn>
										</div>

									</v-card>
								</v-tab-item>
							</v-tabs-items>
						</v-card>
					</div>
				
					
				</template>

				 <template v-slot:no-data>
					<v-alert :value="true" color="error" icon="warning">
					Sorry, there are no services to display here :(
					</v-alert>
            	</template>

				<v-alert slot="no-results" :value="true" color="error" icon="warning">
					Your search for "{{ search }}" found no results.
				</v-alert>
			</v-data-table> 
				<div v-show="show_spinner" style="position:fixed; left:50%;">	
					<intersecting-circles-spinner :animation-duration="1200" :size="50" :color="'#0066ff'" />              		
				</div>  
        </v-card>
      </v-flex>
    </v-layout>
</template>
<script>
import VuePerfectScrollbar from 'vue-perfect-scrollbar'
import axios from 'axios'
import FunctionForm from '@/components/forms/FunctionForm'
import { IntersectingCirclesSpinner } from 'epic-spinners'
import Services from '../components/services';
/* eslint-disable */
export default {
	mixins:[Services],
	components: {
		FunctionForm,
		VuePerfectScrollbar,
		IntersectingCirclesSpinner,
	},
	props: {
		openFaaS: {}
	},
	data: () => ({
		size: 'lg',
		view: 'grid',
		show_spinner: true,
		show_alert: false,
		headers: [
			{ text: 'SERVICE', sortable: false, align: 'center', value: 'service' },
			{ text: 'CONTAINER', sortable: false, align: 'center', value: 'container' },
			{ text: 'CPU', sortable: false, align: 'center',value: 'cpu' },
			{ text: 'MEMORY', sortable: false, align: 'center', value: 'memory' },
			{ text: '', sortable: false, align: 'center', value: 'actions' },
			{ text: '', sortable: false, align: 'center', value: 'logs' },
			{ text: '', sortable: false, align: 'center', value: 'expand' },
		],
		loading: true,
		search: '',
		services:[],
		env_Vars: {},		
		index: '',
		expand: false,
		expand_icon: 'expand_more', 
		model: 'tab-service',
		disable_form: true,
		disable_storage: true,
		params_delete: '', 
	}),
  	methods: {
		showEnvVars(value){
			setTimeout(function(){
				$('#json-renderer').jsonViewer(value);

			},100)
		},
		show(id){
			$(".tab-pane").removeClass("show active")
			$(".nav-link").removeClass("show active")
			$("#"+id).addClass("show active")
			$("#"+id+"-tab").addClass("show active")
		},
		handleSHOW(){
		  this.show_spinner = true      
		},
		edit_service(){
			this.disable_form = false;
		},
		edit_storage(){
			this.disable_storage = false;
		},
		editFunction (func) {
			const index = this.services.indexOf(func)
			let servInfo = {
				editionMode: true,
				name: this.services[index].service,
				image: this.services[index].container,
				input: this.services[index].inputs,
				output: this.services[index].outputs,
				log_Level: this.services[index].logLevel,
				envVars: this.services[index].envVars,
				cpu: this.services[index].cpu,
				script: this.services[index].script,
				memory: this.services[index].memory,
				storage_provider: this.services[index].storage
			
			}
			window.getApp.$emit('FUNC_OPEN_MANAGEMENT_DIALOG', servInfo)
		},    
		deleteFunction(serv, servName) {      
			this.index = this.services.indexOf(serv);
			this.params_delete = {deleteService: servName};
			if (confirm('Are you sure you want to delete this function?')) {
				this.deleteServiceCall(servName,this.deleteServiceCallBack);
				this.show_spinner = true
			}
		},
		deleteServiceCallBack(response){
			this.show_spinner == false
			if (response.status == 204) {
				this.services.splice(this.index, 1)
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `Function ${this.params_delete.deleteService} was deleted`, color: 'success' })           
				window.getApp.$emit('FUNC_GET_FUNCTIONS_LIST')
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
			}
		},
		listServicesCallback(response) {
			console.log(response)
			if(response.status == 200){
				this.show_spinner = false;
				this.services = Object.assign(this.services, response.data); 
				this.services = response.data.map((serv) => {
					return {
						service: serv.name,
						container: serv.image,
						cpu: serv.cpu,
						logLevel: serv.log_level,
						envVars: serv.environment,
						memory: serv.memory,
						inputs: serv.input,
						outputs: serv.output,
						storage: serv.storage_provider,
						script: serv.script
					}
				})				
				this.loading = false;   

			}else{
				this.show_alert = true;
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response.data, color: 'error' })

			}
				
		},
		goLogs(service_name){
			this.$router.push({name: "Logs", params:{serviceName: service_name}})
		},
		bottomVisible() {
			const scrollY = window.scrollY
			const visible = document.documentElement.clientHeight
			const pageHeight = document.documentElement.scrollHeight
			const bottomOfPage = visible + scrollY >= pageHeight
			return bottomOfPage || pageHeight < visible
        },
		
  	},
	created: function () {
		window.getApp.$on('FUNC_GET_FUNCTIONS_LIST', () => {
			this.listServicesCall(this.listServicesCallback)
		})
	},
	mounted: function () {
		$('.div-list-content').scroll(function () { 
                // this.bottom = this.bottomVisible()
            }.bind(this));
		this.listServicesCall(this.listServicesCallback)
	}
}
</script>
<style lang="stylus" scoped>
.openFaas {
    background-color: #e6e7e8 !important;
  }
.custom-padding{
	padding-bottom: 0px;
}

.div-list-content{
	max-height: 350px;
	background-color: #ffffff;
	overflow-y: auto;
}

.btn-circle {
    width: 30px;
    height: 30px;
    padding: 6px 0px;
    border-radius: 15px;
    text-align: center;
    font-size: 12px;
    line-height: 1.42857;
}

  .media
    &-cotent--wrap
    &-menu
      min-width: 260px
      border-right: 1px solid #eee
      min-height: calc(100vh - 50px - 64px);
    &-detail
      min-width: 300px
      border-left: 1px solid #eee
</style>
