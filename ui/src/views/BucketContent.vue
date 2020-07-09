<template>
    <v-layout wrap v-show="showBucketContent" id="bucketContent" class="mb-1">      
		<v-flex xs12>
			<v-card>
			<v-card-title primary-title>
				<v-flex xs12 sm4>
				<span class="headline">Bucket <b>{{bucketName}}</b>: </span>
				</v-flex>
				<v-flex xs12 sm8 row>
				<v-text-field
					v-model="search"
					append-icon="search"
					label="Search"
					single-line
					hide-details
				></v-text-field>
				</v-flex>
			</v-card-title>
			<input-file :bucketName="bucketName" :currentPath="current_path" @SHOWSPINNER="handleSHOW()"></input-file>
			</v-card>
		</v-flex>

		<v-dialog
			v-model="dialog_photo"
			max-width="890"
		>
			<v-card>
				<v-card-title
					primary-title
				>
					Preview
				</v-card-title>
			<v-card-actions>
				<v-img :src="url_image"></v-img>
			</v-card-actions>
			<v-card-actions style="justify-content:center">
			
				<v-btn
					
					color="green lighten-2"
					dark
					@click="dialog_photo = false"
				>
				Cancel
				</v-btn>
			</v-card-actions>
			</v-card>

		</v-dialog>

		<v-flex>
			<div class=row style="margin-left:2rem">
				<div style="margin-top:4px;">
					<a href="javascript:void(0)" data-path="plantas" data-name="plantas" @click="fetchData({'path':'','name':bucketName})">{{bucketName}}</a><span>/</span>
				</div>
				<div style="margin-top:4px;" id="path_com"></div>
				<div style="margin-top:4px;" v-show="newFolder">
					<input type="text" id="folderNew" style="border:none;" v-on:blur="handleBlur" v-model="nameFolder" ref="newFolderFocus" v-on:keyup.enter="createFolder()">
				</div>
				<div v-show="newFolder==false" style="margin:0px; padding:0px;">
									
					<v-tooltip bottom>
						 <template v-slot:activator="{ on, attrs }">
							<v-btn 
							icon
							style="padding:0px;margin:0px;" 
							flat small dark 
							color="primary" 
							v-bind="attrs"
							v-on="on"
							@click='addFolder()'>
								<v-icon>create_new_folder</v-icon>
							</v-btn>
						</template>
						<span>Create new path</span>
					</v-tooltip>
				</div>
			</div>
		</v-flex>

		<v-flex row xs12>
			<v-slide-y-transition>
				<v-toolbar class="v-toolbar v-toolbar--fixed" id="actionBar"
							v-show="selectedItemsToolbar" color="primary" text-color="white"
							height="90px">
					<v-card flat color="primary" class="white--text">
					<v-card-title>
						<v-icon color="white" left>check_circle</v-icon>
						<span> &nbsp; {{selectedItems}} selected items</span>
					</v-card-title>
					</v-card>
					<v-spacer></v-spacer>
					<v-dialog v-model="dialog.visible" persistent max-width="290">
					<v-btn slot="activator" outline color="white">Delete selected</v-btn>
					<v-card>
						<v-card-title class="headline">
						<v-flex>
							<v-icon color="warning">warning</v-icon>
							Are you sure you want to delete?
						</v-flex>
						</v-card-title>
						<v-card-text>This cannot be undone!</v-card-text>
						<v-card-text v-show="dialog.deleting">
						Deleting files
						<v-progress-linear
							indeterminate
							color="primary"
							class="mb-0"
						></v-progress-linear>
						</v-card-text>
						<v-divider></v-divider>
						<v-card-actions>
						<v-spacer></v-spacer>
						<v-btn color="error" flat @click.native="dialog.visible = false">No</v-btn>
						<v-btn color="success" flat @click.native="removeSelectedFiles">Yes</v-btn>
						</v-card-actions>
					</v-card>
					</v-dialog>
					<v-btn outline color="white" @click.native="downloadFile">{{ downloadTitle }}</v-btn>
					<v-btn fab icon>
					<v-icon color="white" @click="closeActionsBar">cancel</v-icon>
					</v-btn>
				</v-toolbar>
			</v-slide-y-transition>
		</v-flex>
		<v-flex xs12 >
			<v-data-table
				v-model="selected"
				:headers="headers"
				:items="files"
				class="elevation-1"
				item-key="name"
				:search = "search"
				:pagination.sync="pagination"
				select-all
				v-show="!show_spinner"
				:rows-per-page-items="rowsPerPageItems"
			>
				<template slot="items" slot-scope="props">
					<tr >
					<th>
						<v-checkbox
						:input-value="props.all"
						:indeterminate="props.indeterminate"
						primary
						hide-details
						@click.native="toggleAll"
						></v-checkbox>
					</th>
					<th
						v-for="header in props.headers"
						:key="header.text"
						:class="['column sortable', pagination.descending ? 'desc' : 'asc', header.value === pagination.sortBy ? 'active' : '']"
						@click="changeSort(header.value)"
					>
						<v-icon small>arrow_upward</v-icon>
						{{ header.text }}
					</th>
					</tr>
				</template>
				<template slot="items" slot-scope="props" expand="true">
					<tr :active="props.selected"  @mouseover="c_index=props.index" @mouseleave="c_index=-1" >
					<td class="justify-center" @click="props.selected = !props.selected" >
						<v-icon v-show="c_index!=props.index && props.selected != true" :color="props.item.color">{{props.item.icon}}</v-icon>
						<v-checkbox
							:input-value="props.selected"
							primary
							hide-details
							v-show ="props.index==c_index || props.selected == true"							
						></v-checkbox>
					</td>
					<td v-if="props.item.icon =='folder'" @click="fetchData({'path':props.item.path, 'name':props.item.name})" class="text-xs-center pointer">{{ props.item.name }}</td>
					<td v-else class="text-xs-center">{{ props.item.name }}</td>
					<td class="text-xs-center">{{ findFilesize(props.item.size) }}</td>
					<td class="text-xs-center">{{ findDate(props.item.lastModified) }}</td>
					<td class="justify-center">
						<v-icon style="margin-right:10px;" v-if="props.item.icon=='insert_photo'" small @click="previewFile(props.item)" color="blue darken-2">visibility</v-icon>
						<v-icon style="margin-right:10px;" v-if="props.item.icon!='insert_photo'" small color="grey">visibility_off</v-icon>
						<v-icon small @click="removeFile(props.item)" color="red darken-2">delete_forever</v-icon>
					</td>
					</tr>
				</template>
				<template slot="no-data">
					
					<v-alert v-show="show_alert" :value="true" color="error" icon="warning">
						Sorry, there are no files to display in this bucket :(
					</v-alert>
					
				</template>
				<v-alert slot="no-results" :value="true" color="error" icon="warning">
					Your search for "{{ search }}" found no results.
				</v-alert>
			</v-data-table>
      		<div v-show="show_spinner" style="position:fixed; left:50%;">	
					<intersecting-circles-spinner :animation-duration="1200" :size="50" :color="'#0066ff'" />              		
			</div>
		</v-flex>
		
		<v-layout xs12 align-end justify-end row id="create">
			<v-speed-dial
			class="fixed-dial"
			v-model="speedDial.fab"
			:top="false"
			:bottom="true"
			:right="true"
			:left="false"
			direction="top"
			:open-on-hover="false"
			transition="scale-transition"
			:absolute="false"
						
			>
				<v-btn
					slot="activator"
					v-model="speedDial.fab"
					color="blue darken-2"
					dark
					fab
				>
					<v-icon>cloud_queue</v-icon>
					<v-icon>close</v-icon>
				</v-btn>
				<v-btn
					fab
					dark
					small
					color="green"
					@click="menu = true"
				>
					<v-icon>add</v-icon>
				</v-btn>
				<v-btn
					fab
					dark
					small
					color="red"
					@click="removeBucket(bucketName)"
				>
					<v-icon>delete</v-icon>
				</v-btn>
			</v-speed-dial>
		</v-layout>
		<v-container xs12 grid-list-xl id="createMenu">
			<v-layout row justify-space-between>
				<v-flex xs3 offset-xs8>
					<v-card v-show="menu">
						<v-flex xs12>
							<v-text-field
								label="Bucket name"
								v-model="newBucketName"
							></v-text-field>
							<span v-show="error" style="color: #cc3300; font-size: 12px;"><b>Bucket name is required</b></span>
						</v-flex>
						<v-card-actions>
							<v-spacer></v-spacer>
							<v-btn color="error" flat @click="menu = false">Cancel</v-btn>
							<v-btn color="success" flat @click="createBucket(newBucketName)">Save</v-btn>
						</v-card-actions>
					</v-card>
				</v-flex>
			</v-layout>
		</v-container>
    </v-layout>
</template>
<script>
import InputFile from '@/components/widgets/InputFile'
import axios from 'axios'
import moment from 'moment'
import filesize from 'filesize'
import { IntersectingCirclesSpinner } from 'epic-spinners'
import { saveAs } from 'file-saver'
import Services from '../components/services'
export default {
	mixins:[Services],
	components: {
		InputFile,
		IntersectingCirclesSpinner,
	},
	props: {
		bucketName: {
		type: String,
		default: '',
		},
		
	},
	data: function () {
		return {
		error: false,
		allData:[],
		paths:[],
		dialog_photo: false,
		url_image:"",
		path_to:'',
		tabs_input_output: 'tab-input',
		select_tab: 'input',
		moment : moment,
		prefix: "",
		c_index: -1,
		filesize : filesize,
		showBucketContent: false,
		show_spinner: true,
		show_alert: false,
		toDownload:[],
		search: '',
		current_path:'',
		rowsPerPageItems: [10, 20, 30, 40, {"text":"$vuetify.dataIterator.rowsPerPageAll","value":-1}],
		pagination: {
			sortBy: 'name',
			rowsPerPage: 20
		},
		headers: [
				{
				text: 'Name',
				align: 'left',
				align: 'center',
				value: 'name',				
				},
				{ text:'Size', align: 'center',value: 'size' },
				{ text:'Last Modified',align: 'center',  value: 'lastModified'},
				{ text:'', align: 'center',value: 'actions'}
			],
		selected: [],		
		files: [],
		stream: '',
		speedDial: {
			fab: false
		},
		uploadingFile: {
			loading: false
		},
		dialog: {
			visible: false,
			deleting: false
		},
		menu: false,
		newBucketName: '',
		index_remove:'',
		file_name_remove: '',
		newFolder: false,
		nameFolder: ''
		}
  	},
	created: function () {
		/**
		 *  Add the new file uploaded to list
		 */   
		
		this.current_path = ''
		window.getApp.$on('FILE_UPLOADED', (file) => {
			this.addFileToList(file)
		})
		window.getApp.$on('GET_BUCKET_LIST', () => {      
			this.fetchData({'path':this.current_path, 'name':''})
		})  
		
	},
	mounted: function () {
		
		this.current_path = ''
		var params = {}
		params['name'] = this.bucketName;
		params['path'] = '';
		this.bucketExistCall(params,this.bucketExistCallBack)		
	},
	watch: {
		// $route: 'fetchData',
		$route(){
			var params = {}
			params['name'] = this.bucketName;
			params['path'] = '';
			this.fetchData(params)
		}, 
		"tabs_input_output"(val){
			var params = {'name': this.bucketName}
			if (val == 'tab-input') {
				this.select_tab='input'		
				this.bucketExistCall(params,this.bucketExistCallBack)
				
			}else if (val == 'tab-output') {
				this.select_tab='output'		
				this.bucketExistCall(params,this.bucketExistCallBack)
			}

		}
	},
  	methods: {
		bucketExistCallBack(response){
			if (response == "success"){
				this.showBucketContent = true
		      	this.fetchData({'path':'','name':this.bucketName})
			}else{
				let err = {
					message: `The ${this.bucketName} bucket does not exist`
				}
		      	throw err
			}
		},
		previewFile(item){
			var params_preview = {
				'bucketName':this.bucketName,
				'fileName': item.path
			}
			this.previewFileCall(params_preview,this.previewFileCallBack)
		},
		previewFileCallBack(response){
			this.url_image=response
			this.dialog_photo = true
		},

		findFilesize(item){
			if(item == ""){
				return ""
			}else{
				return (this.filesize(item))
			}

		},
		findDate(item){
			if(item == ""){
				return ""
			}else{
				return ( this.moment(item).format("YYYY-MM-DD HH:mm") )
			}

		},

		handleSHOW(){
		this.show_spinner = true      
		},
		/**
		 * Add file received from inputFile component
		 * @param file
		 */

		handleBlur(){
			this.newFolder = false;
			this.nameFolder = '';
		},

		addFolder(){
			// var newPath = this.currentPath + 
			this.newFolder = true;
			this.$nextTick(() => this.$refs.newFolderFocus.focus())			
		},
		createFolder(){
			this.files = []
			this.showPath(this.current_path+'/'+this.nameFolder)
			if(this.current_path==""){
				this.current_path = this.nameFolder
			}else{
				this.current_path = this.current_path+'/'+this.nameFolder
			}
			this.nameFolder = ''
			this.newFolder = false
			// this.prefix = this.current_path+'/'+this.nameFolder
		},
		
		showPath(path){
			var _path = path.split('/');
			var string = '';
				var link = [];
			for(var i=0; i<_path.length;i++){
				var index = '';
				if(_path[i]!=''){
					var temp_path =  ""
					for(var j=0; j<=i; j++){
						temp_path += _path[j] + '/'
					}
					temp_path = temp_path.substring(0,temp_path.length-1)
					string = '<a href="javascript:void(0)" class="path_com" data-path="'+temp_path+'" data-name="'+_path[i]+'">'+_path[i]+'</a>' + '<span >/</span>'
					link.push(string)
				}				
			}
			$("#path_com").html(link)
			var _this = this
			$(".path_com").on("click",function(){
				_this.fetchData({'path':$(this).attr('data-path'),'name':$(this).attr('data-name')})
			})


		},
		addFileToList (file) {
			var fileExist = false;      
			for (var i=0; i < this.files.length; i++){
				if (this.files[i].etag == file.etag){          
				fileExist = true;
				}        
			}       
			if (!fileExist) {
				this.files.push(file);
			}
		},
		toggleAll () {
		if (this.selected.length) this.selected = []
			else this.selected = this.files.slice()
		},
		changeSort (column) {
			if (this.pagination.sortBy === column) {
				this.pagination.descending = !this.pagination.descending
			} else {
				this.pagination.sortBy = column
				this.pagination.descending = false
			}
		},
		fetchData (value) {
			this.selected = []
			var prefix = value.path			 
			if (prefix == undefined) {
				this.prefix = ""				
			}else{
				this.prefix = prefix
			}
			
			this.current_path = value.path
			this.showPath(this.current_path)
			var params = {}
			params['name'] = this.bucketName;
			params['prefix'] = this.prefix;
			this.getBucketFilesCall(params,this.getBucketFilesCallBack)			
		},
		getBucketFilesCallBack(response) {
			this.search = ''
			this.paths=[]
			this.files = [] 
			this.show_spinner = false;
			if (response.files.length == 0){
				this.show_alert = true;
			}else{
				this.show_alert = false;
			}
			if(response.err != ""){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: response,
					color: 'error'
				})   
			}
			if(response.files != ""){
				this.allData = response.files
				
				var first = []
				for (let index = 0; index < response.files.length; index++) {
					var element = response.files[index].name;
					this.paths.push(element.split("/"))
					
				}
				var complete_path = []
				if (this.prefix != ""){
					var _prefix = this.prefix.split("/");
					if(_prefix.length>1){
						var _path = this.prefix;
						
					}
					for (let z = 0; z < this.paths.length; z++) {
						if(_path){
							for (let _i = 0; _i < _prefix.length; _i++) {
								this.paths[z].splice(0,1)
							}
						}else{
							if(this.prefix == this.paths[z][0]){
								complete_path[z]=this.paths[z][0]+'/'+this.paths[z][1]
								this.paths[z].splice(0,1)
							}
						}
						
					}
				}
			
				for (let i = 0; i < this.paths.length; i++) {
					var first_path = this.paths[i][0];
					var before = -1
					if(i>0){
						before = i-1
					}

					if(i == 0){
						var extension = this.getFileExtension1(first_path)
						if (extension == undefined){
							var folder = {
								name: first_path,
								path: (_path)?_path+'/'+first_path:((complete_path[i])?complete_path[i]:first_path),
								icon: 'folder',
								color: "green",
								lastModified: "",
								size: ""
							}
							first.push(folder)
						}else{
							var file = {
								name: this.paths[i][0],
								path: this.allData[i].name,
								icon: "insert_photo",
								color: "blue",
								lastModified: response.files[i].lastModified,
								size: response.files[i].size,
							}
							first.push(file)
						}
						
					}else if(first_path != this.paths[i-1][0]){
						var extension = this.getFileExtension1(first_path)
						if (extension == undefined){
							var folder = {
								name: first_path,
								path: (_path)?_path+'/'+first_path:((complete_path[i])?complete_path[i]:first_path),
								icon: 'folder',
								color: "green",
								lastModified: "",
								size: ""
							}
							first.push(folder)
						}else{
							var file = {
								name: this.paths[i][0],
								path: this.allData[i].name,
								icon: 'insert_photo',
								color: "blue",
								lastModified: response.files[i].lastModified,
								size: response.files[i].size,
							}
							first.push(file)
						}						
					}
					
				}				
				this.files=first   
				this.pagination.page = 1 
				
			}              
		},
		getFileExtension1(filename) {
			return (/[.]/.exec(filename)) ? /[^.]+$/.exec(filename)[0] : undefined;
		},
		closeActionsBar () {
			this.selected = []
		},
		downloadFile(){
			// let toDownload = []
			var _this = this;
			var type_file = []
			var folder_file = false
			this.selected.map((sel) => {
				return type_file.push(sel.icon)
			})

			for (let i = 0; i < type_file.length; i++) {
				if (type_file[i]=="folder") {
					folder_file = true
				}
			}

			if (folder_file == true) {
				folder_file = false
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
						text: "It is not possible to download a complete directory, you must select only files.",
						color: 'error'
					}) 			
			}else {

				this.selected.map((sel) => {
					return _this.toDownload.push(sel.path)
				})    

				if (this.selected.leght == 1) {
					var params = {'bucketName': this.bucketName, "fileName": this.toDownload, "select": this.selected.length, "response_type": 'blob'}
				} else{
					var params = {'bucketName': this.bucketName, "fileName": this.toDownload, "select": this.selected.length, "response_type": 'arraybuffer'}
				}
				this.downloadFileCall(params,this.downloadFileCallBack)
			}
		},
		downloadFileCallBack(response){
			var _this = this;
			if (this.selected == 1){
				const url = window.URL.createObjectURL(new Blob([response.data]))
				const link = document.createElement('a')
				link.href = url
				link.setAttribute('download', this.toDownload[0]) //or any other extension
				document.body.appendChild(link)
				link.click()

			}else {
				response.generateAsync({ type: "blob" }).then(blob => saveAs(blob, "files"));
				this.closeActionsBar();
			}
		},
		removeSelectedFiles () {      
			this.dialog.deleting = true
			let toRemove = []
			let toRemoveSelect = []
			var folder_select = false
			this.selected.map((sel) => {
				return toRemove.push(sel.path)
			})
			this.selected.map((sel) => {
				return toRemoveSelect.push(sel.icon)
			})
			for (let i = 0; i < toRemoveSelect.length; i++) {
				if(toRemoveSelect[i]=="folder"){
					folder_select = true
				}
				
			}
			if(folder_select == true){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: "Error: Cannot delete a folder with files",
					color: 'error'
				})
				this.dialog.deleting = false
				this.dialog.visible = false
			}else{
				var params_remove_multiple	= {
					'bucketName': this.bucketName,
					'fileName': toRemove
				}	
				this.removeFileCall(params_remove_multiple, this.removeFileCallBackMultiple)		
			}
		},
		removeFileCallBackMultiple(response){
			if(response == "success"){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: `Files deleted correctly`,
					color: 'success'
					})        
					this.selected.map((sel) => {
					return this.files.splice(this.files.indexOf(sel), 1)
				})
				this.dialog.deleting = false
				this.dialog.visible = false
				this.closeActionsBar()
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: response,
					color: 'error'
				})
			}
		},
		removeFile (file) {
			if(file.icon == "folder"){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
						text: "Error: Cannot delete a folder with files",
						color: 'error'
				})
			}else {
				this.file_name_remove = file.name
				this.index_remove = this.files.indexOf(file)
				var params_remove = {
					'bucketName': this.bucketName,
					'fileName': [file.path]
				}
				if (confirm('Are you sure you want to remove this file?')) {
					this.removeFileCall(params_remove, this.removeFileCallBack)
				}
			}
		},
		removeFileCallBack(response){
			if(response=="success"){
				this.files.splice(this.index_remove, 1)
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: `File ${this.file_name_remove} deleted correctly`,
					color: 'success'
				})          
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: response,
					color: 'error'
				})
			}
		},
		
		createBucket (name) {
			this.error = false;
			var params_create = {
				'name': this.newBucketName
			}
			if(this.newBucketName.length > 0){
				this.createBucketCall(params_create,this.createBucketCallback)
			}else {
				this.error = true;
			}
		},
		createBucketCallback (response) {
			if(response == 'success'){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
				text: `Bucket ${name} has been successfully created`,
				color: 'success'
				})
				window.getApp.$emit('REFRESH_BUCKETS_LIST')
			}else if (response.code=='BucketAlreadyOwnedByYou'){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: "Error: The bucket already exists",
					color: 'error'
				})
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
				text: response,
				color: 'error'
				})
			}
			this.menu = false
			this.newBucketName = ''
			
		},
		
		removeBucket (name) {
			if (confirm('Are you sure you want to remove this bucket?')) {
				this.removeBucketCall(name,this.removeBucketCallBack)
			}

		},
		removeBucketCallBack(response){
			if (response == 'success') {
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
				text: `Bucket ${name} has been successfully deleted`,
				color: 'success'
				})
				this.$router.push({name: "Functions"}) 
				window.getApp.$emit('REFRESH_BUCKETS_LIST')
			}else if (response.code == "BucketNotEmpty"){
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
					text: "Error: The bucket is not empty",
					color: 'error'
				})
			}else{
				window.getApp.$emit('APP_SHOW_SNACKBAR', {
				text: err.message,
				color: 'error'
				})
			}
		}
  	},
  computed: {
    selectedItems () {
      return this.selected.length
    },
    selectedItemsToolbar () {
      return this.selected.length > 0
    },
    downloadTitle () {
      return this.selected.length === 0 || this.selected.length === 1 ? 'Download object' : 'Download all as zip'
    }
  }
}
</script>

<style scoped>
  /* This is for documentation purposes and will not be needed in your application */
	#create .v-speed-dial {
		position: absolute;
	}

	#create .v-btn--floating {
		position: relative;
	}
	#actionBar {
		z-index: 4;
	}
	.pointer:hover{
		cursor: pointer;
	}

	.folderbtn.btn__content {
		padding: 0;
		}

	.folderbtn.card__actions .btn {
	min-width: 0;
	}
	input:focus{
    outline: none;
	color: #1976d2;
	}
	.example {
    	min-height: 180px;
  	}

  	.md-speed-dial {
    	margin: 0 24px 0 8px;
  	}
	  .fixed-dial{
		position: fixed !important;
		bottom: 40px;
		right: 15px;
	  }
</style>
