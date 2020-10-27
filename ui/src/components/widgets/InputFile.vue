<template>
  <v-layout align-center row>
    <v-flex xs12 sm12 md6>
      <v-btn
        color="primary"
        class="white--text"
        @click.native="addFiles()"
      >
        Select files
        <v-icon right dark>note_add</v-icon>
      </v-btn>
      
      <v-btn
        :loading="showUploading"
        :disabled="showUploading"
        color="teal"
        class="white--text"
        @click.native="submitFiles()"
      >
        Upload
        <v-icon right dark>cloud_upload</v-icon>
      </v-btn>
      <v-btn flat icon color="blue" @click="updateFile()">
      		<v-icon>autorenew</v-icon>
    	</v-btn>
    </v-flex>
    <v-flex xs12 sm12 md6 v-show="showSelectedFiles" id="selectedList">
        <input type="file" id="files" ref="files" multiple v-on:change="handleFilesUpload()"/>
        <v-list two-line subheader dense>
          <v-subheader inset>Files</v-subheader>
          <v-list-tile
            v-for="(file, key) in files"
            :key="key"
            avatar
            @click.stop=""
          >
              <v-progress-circular
                indeterminate
                color="teal"
                v-show="show"
              >
              </v-progress-circular>
            
            <v-list-tile-content>
              <v-list-tile-title>{{file.name}}</v-list-tile-title>
              <v-list-tile-sub-title>{{ moment(file.lastModified).format("YYYY-MM-DD HH:mm") }}</v-list-tile-sub-title>
            </v-list-tile-content>

            <v-list-tile-action>
              <v-btn icon ripple @click="removeFile(key)">
                <v-icon color="red lighten-1">remove_circle_outline</v-icon>
              </v-btn>
            </v-list-tile-action>
          </v-list-tile>
        </v-list>
    </v-flex>
  </v-layout>
</template>

<script>
import AWS from 'aws-sdk'
import axios from 'axios'
import moment from 'moment'
import Services from '../services.js'

export default {
  mixins:[Services],
  name: 'InputFile',  
  props: {
    accept: {
      type: String,
      default: '*'
    },
    label: {
      type: String,
      default: 'Please choose...'
    },
    required: {
      type: Boolean,
      default: false
    },
    disabled: {
      type: Boolean,
      default: false
    },
    multiple: {
      type: Boolean, // not yet possible because of data
      default: false
    },
    uploading: {
      type: Boolean,
      default: false
    },
    bucketName: {
      type: String,
      default: ''
    },
    currentPath: {
      type: String,
      default: ''
    }
  },
  data () {
    return {
      moment : moment,
      files: [],
      showUploading: false,
      show: false,
      filename_up:''
    }
  },
  methods: {
    updateFile(){
      this.$emit("SHOWSPINNER",true)
      window.getApp.$emit('GET_BUCKET_LIST')
    },
    /**
     * Adds a file
     */
    addFiles () {     
      this.$refs.files.click()
    },

    /**
     * Submits files to the server
     */
    submitFiles () {
      /*
        Initialize the form data
      */
      
      /*
        Iteate over any file sent over appending the files
        to the form data.
      */
	 	
		for (let i = 0; i < this.files.length; i++) {
      var filename = ''
      this.filename_up = this.files[i].name
			if(this.currentPath == ''){
				filename = this.files[i].name
			}else{
				filename = this.currentPath+'/'+this.files[i].name
      }
      this.showUploading = true;
      this.show = true
			var params={
				"bucketName": this.bucketName,
				"file": this.files[i],
				"file_name": filename
      }
			this.uploadFileCall(params, this.uploadFileCallBack)
    
		}
    },
    uploadFileCallBack(response){
		var _this = this
		if (response!="uploaded"){
			window.getApp.$emit('APP_SHOW_SNACKBAR', {
            	text: `Error uploading file ${response}`,            
            	color: 'error'
          	})
		}else {

      this.showUploading = false;
      this.show = false;
        window.getApp.$emit('APP_SHOW_SNACKBAR', {
          text: `The ${this.filename_up} file has been successfully uploaded`,
          color: 'success'
        })          
        window.getApp.$emit('GET_BUCKET_LIST') 
        this.$refs.files.value = null
        this.files = []
      }
	},
    /**
     * Handles the uploading of files
     */
    handleFilesUpload () {
	  let uploadedFiles = this.$refs.files.files  
      /*
        Adds the uploaded file to the files array
      */
      for (let i = 0; i < uploadedFiles.length; i++) {
        this.files.push(uploadedFiles[i])
      }
     
    },
    /**
     * Removes a select file the user has uploaded
     * @param key
     */
    removeFile (key) {
      this.files.splice(key, 1)
    },
    /**
     * Upload file to minio server
     * @param file
     */
  },
  computed: {
    showSelectedFiles () {
      return this.files.length > 0
    }
  }
}
</script>

<style>
  input[type="file"]{
    position: absolute;
    top: -500px;
  }

  #selectedList {
    max-height: 200px;
    overflow-y: auto;
  }
</style>
<style lang="stylus" scoped>
  .v-progress-circular
    margin: 1rem
</style>
