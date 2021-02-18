<template>
  <v-app id="login" class="teal darken-1">
    <v-content>
      <v-container fluid fill-height>
        <v-layout align-center justify-center>
          <v-flex xs12 sm8 md4 lg4>
            <v-card class="elevation-1 pa-3">
              <v-card-text>
                <div class="layout column align-center">
                  <img src="@/assets/logo.png" alt="Vue Material Admin" width="120" height="120">
                  <h1 class="flex my-4 teal--text">OSCAR ADMIN</h1>
                </div>
                <v-form>
                  <v-text-field append-icon="person" name="login" label="Login" type="text"
                                v-model="model.username"></v-text-field>
                  <v-text-field append-icon="lock" name="password" label="Password" id="password" type="password"
                                v-model="model.password" v-on:keyup="bindLogin()"></v-text-field>
                </v-form>
              </v-card-text>
              <v-card-actions>
                <!-- <v-btn icon>
                  <v-icon color="blue">fa fa-facebook-square fa-lg</v-icon>
                </v-btn>
                <v-btn icon>
                  <v-icon color="red">fa fa-google fa-lg</v-icon>
                </v-btn>
                <v-btn icon>
                  <v-icon color="light-blue">fa fa-twitter fa-lg</v-icon>
                </v-btn> -->
                <v-spacer></v-spacer>
                <v-btn color="teal" dark @click.native="login()" :loading="loading">Login</v-btn>
              </v-card-actions>
            </v-card>
          </v-flex>
        </v-layout>
      </v-container>
    </v-content>
  </v-app>
</template>

<script>
import Services from '../components/services.js';
export default {
  mixins:[Services],
  data: () => ({
    loading: false,
    model: {
      username: '',
      password: ''
    }, 
    user: "admin",
    pass: "admin",
    
  }),
  created(){
    localStorage.clear();
    localStorage.setItem("authenticated", false);
  },

  methods: {
    bindLogin(){
      event.preventDefault();
      if (event.keyCode === 13) {
        this.login()
      } 
    },
    login () {
      this.loading = true
      var params = {
        'user': this.model.username,
        'password': this.model.password
      }
      this.checkLoginCall(params,this.checkLoginCallback)
      
    },
    getPort(url) {
        url = url.match(/^(([a-z]+:)?(\/\/)?[^\/]+).*$/)[1] || url;
        var parts = url.split(':'),
            port = parseInt(parts[parts.length - 1], 10);
        return port;
    },
    getHost(url) {
       var hostname;
        //find & remove protocol (http, ftp, etc.) and get hostname

        if (url.indexOf("//") > -1) {
            hostname = url.split('/')[2];
        }
        else {
            hostname = url.split('/')[0];
        }

        //find & remove port number
        hostname = hostname.split(':')[0];
        //find & remove "?"
        hostname = hostname.split('?')[0];

        return hostname;
    },
    checkLoginCallback(response){
      if(response == 200){
        var _this = this
          axios({
                method: 'get',
                url: '/system/config',
                auth: {
                    username: this.model.username,
                    password: this.model.password
                }
              }).then(function (response) {
                  var port=_this.getPort(response.data.minio_provider.endpoint)
                  var endpoint_host = _this.getHost(response.data.minio_provider.endpoint)
                  localStorage.setItem("endpoint",endpoint_host)
                  localStorage.setItem("accessKey",response.data.minio_provider.access_key)
                  localStorage.setItem("secretKey",response.data.minio_provider.secret_key)
                  localStorage.setItem("port",port)
                  localStorage.setItem("authenticated", true);
                  localStorage.setItem("user", _this.model.username);
                  localStorage.setItem("password", _this.model.password);
                  _this.$router.push({name: "Functions"}) 
              }).catch(function (error) {
                  console.log(error)
              })
            
      }else if (response == 401){
        this.loading = false
        window.getApp.$emit('APP_SHOW_SNACKBAR', { text: "Username or password is incorrect", color: 'error' })
      }else{
        this.loading = false
        window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
      }
    }
  }

}
</script>
<style scoped lang="css">
  #login {
    height: 50%;
    width: 100%;
    position: absolute;
    top: 0;
    left: 0;
    content: "";
    z-index: 0;
  }
</style>
