<template>
<v-layout row wrap>
  <app-drawer class="app--drawer"></app-drawer>
  <app-toolbar class="app--toolbar"></app-toolbar>
  <v-layout align-center justify-center row wrap fill-height>
    <v-flex xs12>
      <v-card>
        <v-form ref="form" v-model="valid" lazy-validation>
          <v-layout row wrap>
            <v-flex xs12 sm6>
              <v-card flat>
                <v-card-title>
                  <span class="headline">OpenFaaS</span>
                </v-card-title>
                <v-card-text>
                  <v-text-field
                    v-model="openFaaSConfig.endpoint"
                    :rules="rules.endpoint"
                    label="Endpoint"
                    required
                  ></v-text-field>
                  <v-text-field
                    v-model="openFaaSConfig.port"
                    :rules="rules.port"
                    label="Port"
                    required
                  ></v-text-field>
                </v-card-text>
              </v-card>
            </v-flex>
            <v-flex xs12 sm6>
              <v-card flat>
                <v-card-title>
                  <span class="headline">Minio</span>
                </v-card-title>
                <v-card-text>
                  <v-text-field
                    v-model="minioConfig.endpoint"
                    :rules="rules.endpoint"
                    label="Endpoint"
                    required
                  ></v-text-field>
                  <v-layout row justify-space-between>
                    <v-flex xs6>
                      <v-text-field
                        v-model="minioConfig.port"
                        :rules="rules.port"
                        label="Port"
                        required
                      ></v-text-field>
                    </v-flex>
                    <v-flex xs6>
                      <v-checkbox
                        v-model="minioConfig.useSSL"
                        label="Use SSL"
                        minio.useSSL
                      ></v-checkbox>
                    </v-flex>
                  </v-layout>
                  <v-text-field
                    v-model="minioConfig.accessKey"
                    label="Access key"
                    :rules="[rules.required]"
                  ></v-text-field>
                  <v-text-field
                    v-model="minioConfig.secretKey"
                    :append-icon="showMinioSecretKey ? 'visibility_off' : 'visibility'"
                    :rules="[rules.required]"
                    :type="showMinioSecretKey ? 'text' : 'password'"
                    name="minioSecretKey"
                    label="Secret key"
                    @click:append="showMinioSecretKey = !showMinioSecretKey"
                  ></v-text-field>
                </v-card-text>
              </v-card>
            </v-flex>
            <v-flex xs12>
              <v-layout align-center justify-center row>
                <v-btn outline color="success" @click="submit">
                  Save
                  <v-icon right>save</v-icon>
                </v-btn>
                <v-btn outline color="error" to="dashboard">
                  Cancel
                  <v-icon right>cancel</v-icon>
                </v-btn>
              </v-layout>
            </v-flex>
          </v-layout>
        </v-form>
      </v-card>
    </v-flex>
  </v-layout>
</v-layout>
</template>

<script>
import AppDrawer from '@/components/AppDrawer'
import AppToolbar from '@/components/AppToolbar'
export default {
  components: {
    AppDrawer,
    AppToolbar,    
  },
  props: {
    minio: {},
    openFaaS: {}
  },
  data: () => ({
    valid: true,
    rules: {
      required: value => !!value || 'Required.',
      endpoint: [
        v => !!v || 'Endpoint is required'
      ]
    },
    showMinioSecretKey: false,
    minioConfig: {},
    openFaaSConfig: {}
  }),
  created: function () {
    this.minioConfig = Object.assign({}, this.minio)
    this.openFaaSConfig = Object.assign({}, this.openFaaS)
  },
  methods: {
    submit () {
      if (this.$refs.form.validate()) {
        this.minio.endpoint = this.minioConfig.endpoint
        this.minio.port = Number(this.minioConfig.port)
        this.minio.useSSL = this.minioConfig.useSSL
        this.minio.accessKey = this.minioConfig.accessKey
        this.minio.secretKey = this.minioConfig.secretKey
        this.openFaaS.endpoint = this.openFaaSConfig.endpoint
        this.openFaaS.port = Number(this.openFaaSConfig.port)
        window.getApp.$emit('MINIO_RECONNECT')
        window.getApp.$emit('APP_SHOW_SNACKBAR', { text: 'Configuration saved! :)', color: 'success' })
      }
    }
  }
}
</script>
