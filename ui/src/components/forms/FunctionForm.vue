<template>
	<v-layout row wrap justify-end>    
		<v-btn flat icon color="blue" @click="handleUpdate()">
      		<v-icon>autorenew</v-icon>
    	</v-btn>
		<v-dialog id="vdiag" lazy v-model="dialog" persistent :fullscreen="$vuetify.breakpoint.xs" max-width="50%">
			<v-btn slot="activator" color="teal" dark class="mb-2">
				<v-icon left>add_box</v-icon>
				Deploy New Service
			</v-btn>
			<v-card >
				<v-form ref="form" v-model="form.valid" lazy-validation>
					<v-toolbar flat :color="formColor" class="white--text" style="margin-bottom:10px;">
						<span class="headline" style="width:100%;text-align:center">{{ formTitle }}</span>
					</v-toolbar>
					
					<ul class="nav nav-pills nav-fill" id="myTab" role="tablist" style="padding-right:5px; padding-left:5px;">
						<li class="nav-item" style="margin-rigth:10px;margin-left:10px;">
							<a class="nav-link active" id="home-tab" @click="show('home')" role="tab" aria-controls="home" aria-selected="true">New Service</a>
						</li>
						<li class="nav-item" style="margin-rigth:10px;margin-left:10px;">
							<a class="nav-link" id="profile-tab" @click="show('profile')" role="tab" aria-controls="profile" aria-selected="false">Storage</a>
						</li>  
						<li class="nav-item" style="margin-rigth:10px;margin-left:10px;">
							<a class="nav-link " id="input_output-tab" @click="show('input_output')" role="tab" aria-controls="input_output" aria-selected="false">Input/Output</a>
						</li>
					</ul>
					 
					<v-progress-linear :active="progress.active" :indeterminate="true"></v-progress-linear>
					 
					<div class="tab-content" id="myTabContent">
						<div class="tab-pane tab-pane-content fade show active" id="home" role="tabpanel" aria-labelledby="home-tab">
							<v-card-text>
									<v-layout wrap>
										<div style="width:100%;padding: 0px 10px;">
											<v-flex xs12>
												<v-text-field
													v-model="form.image"
													:rules="form.imageRules"
													:counter="200"
													label="Docker image:"
													required
												></v-text-field>
											</v-flex>

											<v-flex xs12>
												<v-text-field
													v-model="form.name"
													:rules="form.nameRules"
													:counter="200"
													label="Function name"
													required
													:disabled="this.editionMode"
												></v-text-field>
											</v-flex>               
										</div>
										<div class="row" style="width:100%;padding: 0px 10px;"> 	
											<v-flex xs12  md5 text-xs-center>
												<v-btn color="primary" class="white--text" @click.native="addFiles()"> Select a file<v-icon right dark>note_add</v-icon></v-btn>
											</v-flex>

											<v-flex xs12  md2 class="text-xs-center">
												<v-chip>or</v-chip>
											</v-flex>

											<v-flex xs12  md5>
												<div style="margin:10px" class="form-group">                     
												<div class="input-group">
													<input type="text" class="form-control" id="bucketname" autocomplete="off" v-model="url"   placeholder="URL" autofocus  style="border-right: none; border-left:none; border-top:none; hover: "/>                     
													
													<div class="input-group-append mr-2">                        
													<button class="" @click="readurl()" type="button"><v-icon left color="green">check_circle</v-icon></button>
													<button class="" @click="cleanfield()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
													</div>
												</div>            
												</div> 
											</v-flex>

											<v-flex xs12 text-xs-center>
												<span v-show="filerequire" style="color: #cc3300; font-size: 12px;"><b>Select a file or enter a URL</b></span>                   									
											</v-flex>

											
											<v-flex xs12 sm8 offset-sm2 v-show="showSelectedFiles"  id="selectedList" class="text-xs-center">
													<input type="file" id="files" ref="files" hidden=true  v-on:change="handleFilesUpload()"/>								                  									
													<v-list subheader dense >
														<v-subheader inset>File</v-subheader>
														<v-list-tile
														v-for="(file, key) in files"
														:key="key"
														avatar
														@click.stop=""
														>                                    
																<v-progress-circular
																indeterminate
																color="teal"
																v-show="showUploading"
																>
																</v-progress-circular>

																<v-list-tile-content>
																	<v-list-tile-title>{{ filename }}</v-list-tile-title>														
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeFile(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
													<v-card flat>
														<v-card-actions >
														<v-btn
															outline color="indigo"
															round
															small
															@click.native="editSummernote()"
														>
															Edit															
														</v-btn>
														<v-btn
															outline color="indigo"
															round
															small
															@click.native="saveSummernote()"
														>
															Save															
														</v-btn>
														</v-card-actions>
														<v-card-actions>
														
														</v-card-actions>
													</v-card>
											</v-flex>	
											<v-flex xs-12 text-center v-show="editionMode==true && showSelectedFiles == false">
												<span>Note: To edit the script sending in the creation of the service press the Edit button.</span>
												<v-card flat>
														<v-card-actions style="justify-content: center;">
														<v-btn
															outline color="indigo"
															round
															small
															@click.native="editSummernote()"
														>
															Edit															
														</v-btn>
														<v-btn
															outline color="indigo"
															round
															small
															@click.native="saveSummernote()"
														>
															Save															
														</v-btn>
														</v-card-actions>
													</v-card>
											</v-flex>
											<v-flex xs12>
                                            	<!-- <div  v-show="editScript==true" style="white-space: pre-wrap;" class="click2edit text-left"></div>				 -->
												<div class="summernote" style="white-space: pre-wrap;"></div>
                                            	<!-- <div v-show="editScript==true" style="white-space: pre-wrap;" class="click2edit text-left"></div>				 -->
											</v-flex>
									
											<v-flex xs12>
													<v-btn
													outline color="indigo"
													round
													small
													@click.native="collapse()"
												>
													More Options
													<v-icon right dark>{{expand}}</v-icon>
												</v-btn>                    
											</v-flex>
										</div>
									
										<v-flex xs12 id="panel">  								
											<v-container>
												<v-layout row wrap>
													<div class="form-group" style="width:100%">                     
													<div class="input-group">
														<v-flex xs12 sm5>
															<v-text-field
																v-model="form.envVarskey"
																:counter="200"
																label="Environment variables (key)"		
																style="padding-right: 5px;"									
															></v-text-field>
														</v-flex>	
														<v-flex xs10 sm5>
															<v-text-field
																v-model="form.envVarsValue"
																:counter="200"
																label="Environment variables (value)"		
																style="padding-right: 5px;"									
															></v-text-field>
														</v-flex>	
														
														<v-flex xs2 sm2 style="padding-top:20px;" >
															<div  class="input-group-append mr-2">  														                    
																<button class="" @click="includeEnv()" type="button"><v-icon left color="green">check_circle</v-icon></button>
																<button class="" @click="cleanfieldenv()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
															</div>
														</v-flex>
													</div>            
													</div> 
												</v-layout>

												<v-flex xs12 sm6 offset-sm3 v-show="showselectEnv">
													<input type="file" id="envs" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>Env Vars</v-subheader>
														<v-list-tile
														v-for="(enVar,key) in envVars"
														:key="key"
														avatar
														@click.stop=""
														>       

																<v-list-tile-content>
																	<v-list-tile-title>{{key}}:{{envVars[key]}}</v-list-tile-title>
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeEnv(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>
												
												<v-layout row wrap>
													<v-flex xs12 sm5>
														<v-text-field
															v-model="form.limits_cpu"
															:counter="10"
															label="CPU"
															style="padding-right: 5px;"											
														></v-text-field>											
													</v-flex>

													<v-flex xs10 sm5>
														<v-text-field
															v-model="form.limits_memory"
															:counter="10"
															label="Memory"
															style="padding-right: 5px;"																											
														></v-text-field>																						
													</v-flex>  
													<v-flex xs2 sm2 style="padding-top:10px;">
														<select id="classmemory" class="custom-select" >																										
															<option selected value="1">Mi</option>
															<option value="2">Gi</option>															
														</select>
													</v-flex>	

													<v-flex xs12 sm3 style="padding-top:10px;">
														<v-select
															:items="form.log_level"
															label="LOG LEVEL"
															v-model="select_logLevel"
														></v-select>
													</v-flex>	
													
												</v-layout>
											</v-container>								
										</v-flex> 						
									</v-layout>
							</v-card-text>
							<v-card-actions >
								<v-btn @click="closeWithoutSave()" flat color="grey">Cancel</v-btn>
								<v-btn @click="clearHome()" flat color="red">Clear</v-btn>
								<v-spacer></v-spacer>
								<v-btn @click="show('profile')" flat color="success">NEXT</v-btn>
							</v-card-actions>
						</div>

						<div class="tab-pane tab-pane-content fade" id="input_output" role="tabpanel" aria-labelledby="input_output-tab" style="padding-right:3rem; padding-left:3rem;">
							
							<ul class="nav nav-pills nav-fill" id="myTabInOut" role="tablist" style="padding-right:5px; padding-left:5px;">
								<li class="nav-item" style="margin-rigth:10px;margin-left:10px;">
									<a class="nav-link active" id="input-tab" @click="show_input('input')" role="tab" aria-controls="home" aria-selected="true">INPUTS</a>
								</li>
								<li class="nav-item" style="margin-rigth:10px;margin-left:10px;">
									<a class="nav-link" id="output-tab" @click="show_input('output')" role="tab" aria-controls="input_output" aria-selected="false">OUTPUTS</a>
								</li>
							</ul>

							<div class="tab-content" id="myTabContentInOut">
								<div class="tab-pane tab-pane-inout fade show active"  id="input" role="tabpanel" aria-labelledby="input-tab" style="padding-right:3rem; padding-left:3rem;">
										<v-layout row wrap>
											<v-flex xs12>  								
												<v-container>
													<v-flex xs12 text-xs-center>
														<span v-show="showErrorInput" style="color: #cc3300; font-size: 12px;"><b>The Storage Provider and Path fields are required.</b></span>                   									
													</v-flex>
													<div class="form-group" style="width:100%">                     
														<div class="input-group">
															<v-flex class="col-12">
																<v-select
																	v-model="form.storage_provider_in"
																	:items="storages_all"
																	label="Storage Provider"
																	outline
																></v-select>																
															</v-flex>
															<v-flex>
																<v-text-field
																	v-model="form.path_in"
																	:counter="200"
																	label="Path"		
																	style="padding-right: 5px;"									
																></v-text-field>
															</v-flex>															
														</div>            
													</div> 

													<div class="form-group" style="width:100%">                     
														<div class="input-group">
															<v-flex>
																<v-text-field
																	v-model="form.prefix_in"
																	:counter="200"
																	label="Prefix"		
																	style="padding-right: 5px;"									
																></v-text-field>
															</v-flex>	

															<div  class="input-group-append mr-2">  														                    
																<button class="" @click="includePrefixIn()" type="button"><v-icon left color="green">check_circle</v-icon></button>
																<button class="" @click="cleanfieldPrefixIn()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
															</div>
														</div>            
													</div> 

													<v-flex xs12 sm6 offset-sm3 v-show="showselectPrefixIn">
														<input type="file" id="prefixs" hidden="true" multiple />
														<v-list subheader dense>
															<v-subheader inset>Prefixs</v-subheader>
															<v-list-tile
															v-for="(prefix,key) in prefixs_in"
															:key="key"
															avatar
															@click.stop=""
															>                                    
																	

																	<v-list-tile-content>
																		<v-list-tile-title>{{prefix}}</v-list-tile-title>
																	</v-list-tile-content>

																	<v-list-tile-action>
																		<v-btn icon ripple @click="removePrefixIn(key)">
																		<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																		</v-btn>
																	</v-list-tile-action>
															</v-list-tile>
														</v-list>
													</v-flex>



													<div class="form-group" style="width:100%">                     
														<div class="input-group">
															<v-flex>
																<v-text-field
																	v-model="form.suffix_in"
																	:counter="200"
																	label="Suffix"		
																	style="padding-right: 5px;"									
																></v-text-field>
															</v-flex>	
															<div  class="input-group-append mr-2">  														                    
																<button class="" @click="includeSuffixIn()" type="button"><v-icon left color="green">check_circle</v-icon></button>
																<button class="" @click="cleanfieldSuffixIn()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
															</div>
														</div>            
													</div> 

													<v-flex xs12 sm6 offset-sm3 v-show="showselectSuffixIn">
														<input type="file" id="suffix" hidden="true" multiple />
														<v-list subheader dense>
															<v-subheader inset>Suffixs</v-subheader>
															<v-list-tile
															v-for="(suffix,key) in suffixs_in"
															:key="key"
															avatar
															@click.stop=""
															>                                    

																<v-list-tile-content>
																	<v-list-tile-title>{{suffix}}</v-list-tile-title>
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeSuffixIn(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
															</v-list-tile>
														</v-list>
													</v-flex>
												</v-container>
											</v-flex>	
										</v-layout>

										<v-flex xs12 v-show="showselectInput">
											<input type="file" id="inputs" hidden="true" multiple />
											<v-list subheader dense three-line>
												<v-subheader inset>Inputs</v-subheader>
												<v-list-tile
												v-for="(input,key) in inputs"
												:key="key"
												avatar
												@click.stop=""
												style="margin-bottom:10px;"
												>                                    
														<v-list-tile-content>
															<v-list-tile-title style="padding-bottom:20px;">Path: {{input.path}}</v-list-tile-title>
															<v-list-tile-title style="padding-bottom:20px;">Storage_provider: {{input.storage_provider}}</v-list-tile-title>
															<v-list-tile-title style="padding-bottom:20px;">Prefix: {{input.prefix}}</v-list-tile-title>
															<v-list-tile-title style="padding-bottom:25px;">Suffix: {{input.suffix}}</v-list-tile-title> 
														</v-list-tile-content>

														<v-list-tile-action>
															<v-btn icon ripple @click="removeInput(key)">
															<v-icon color="red lighten-1">remove_circle_outline</v-icon>
															</v-btn>
														</v-list-tile-action>
												</v-list-tile>
											</v-list>
										</v-flex>

										<v-card-actions class="text-md-center" >
											<v-spacer></v-spacer>
											<v-btn @click="includeInput()"  color="info">ADD INPUT</v-btn>
											
										</v-card-actions>
								</div>

								<div class="tab-pane tab-pane-inout fade"  id="output" role="tabpanel" aria-labelledby="output-tab" style="padding-right:3rem; padding-left:3rem;">
									<v-layout row wrap>
										<v-flex xs12>  								
											<v-container>
												<v-flex xs12 text-xs-center>
													<span v-show="showErrorOutput" style="color: #cc3300; font-size: 12px;"><b>The Storage Provider and Path fields are required.</b></span>                   									
												</v-flex>
												<div class="form-group" style="width:100%">                     
													<div class="input-group">
														<v-flex class="col-12">
															<v-select
																v-model="form.storage_provider_out"
																:items="storages_all"
																label="Storage Provider"
																outline
															></v-select>																
														</v-flex>
														<v-flex>
															<v-text-field
																v-model="form.path_out"
																:counter="200"
																label="Path"		
																style="padding-right: 5px;"									
															></v-text-field>
														</v-flex>	
														
													</div>            
												</div> 

												<div class="form-group" style="width:100%">                     
													<div class="input-group">
														<v-flex>
															<v-text-field
																v-model="form.prefix_out"
																:counter="200"
																label="Prefix"		
																style="padding-right: 5px;"									
															></v-text-field>
														</v-flex>	

														<div  class="input-group-append mr-2">  														                    
															<button class="" @click="includePrefixOut()" type="button"><v-icon left color="green">check_circle</v-icon></button>
															<button class="" @click="cleanfieldPrefixOut()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
														</div>
													</div>            
												</div> 

												<v-flex xs12 sm6 offset-sm3 v-show="showselectPrefixOut">
													<input type="file" id="prefixs" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>Prefixs</v-subheader>
														<v-list-tile
														v-for="(prefix,key) in prefixs_out"
														:key="key"
														avatar
														@click.stop=""
														>                                    
																

																<v-list-tile-content>
																	<v-list-tile-title>{{prefix}}</v-list-tile-title>
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removePrefixOut(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>



												<div class="form-group" style="width:100%">                     
													<div class="input-group">
														<v-flex>
															<v-text-field
																v-model="form.suffix_out"
																:counter="200"
																label="Suffix"		
																style="padding-right: 5px;"									
															></v-text-field>
														</v-flex>	
														<div  class="input-group-append mr-2">  														                    
															<button class="" @click="includeSuffixOut()" type="button"><v-icon left color="green">check_circle</v-icon></button>
															<button class="" @click="cleanfieldSuffixOut()" type="button" ><v-icon left color="red">cancel</v-icon></button>                        
														</div>
													</div>            
												</div> 



												<v-flex xs12 sm6 offset-sm3 v-show="showselectSuffixOut">
													<input type="file" id="suffix" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>Suffixs</v-subheader>
														<v-list-tile
														v-for="(suffix,key) in suffixs_out"
														:key="key"
														avatar
														@click.stop=""
														>                                    

															<v-list-tile-content>
																<v-list-tile-title>{{suffix}}</v-list-tile-title>
															</v-list-tile-content>

															<v-list-tile-action>
																<v-btn icon ripple @click="removeSuffixOut(key)">
																<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																</v-btn>
															</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>
											</v-container>
										</v-flex>	
									</v-layout>

									<v-flex xs12 v-show="showselectOutput">
										<input type="file" id="inputs" hidden="true" multiple />
										<v-list subheader dense three-line>
											<v-subheader inset>Outputs</v-subheader>
											<v-list-tile
											v-for="(output,key) in outputs"
											:key="key"
											avatar
											@click.stop=""
											style="margin-bottom:10px;"
											>                                    
													<v-list-tile-content>
														<v-list-tile-title style="padding-bottom:20px;">Path: {{output.path}}</v-list-tile-title>
														<v-list-tile-title style="padding-bottom:20px;">Storage_provider: {{output.storage_provider}}</v-list-tile-title>
														<v-list-tile-title style="padding-bottom:20px;">Prefix: {{output.prefix}}</v-list-tile-title>
														<v-list-tile-title style="padding-bottom:25px;">Suffix: {{output.suffix}}</v-list-tile-title> 
													</v-list-tile-content>

													<v-list-tile-action>
														<v-btn icon ripple @click="removeOutput(key)">
														<v-icon color="red lighten-1">remove_circle_outline</v-icon>
														</v-btn>
													</v-list-tile-action>
											</v-list-tile>
										</v-list>
									</v-flex>

									<v-card-actions class="text-md-center" >
										<v-spacer></v-spacer>
										<v-btn @click="includeOutput()"  color="info">ADD OUTPUT</v-btn>
									</v-card-actions>
								</div>
								<v-card-actions >
									<v-btn @click="closeWithoutSave()" flat color="grey">Cancel</v-btn>
									<v-btn @click="cleanfieldInput();cleanfieldOutput()" flat color="red">Clear</v-btn>
									<v-spacer></v-spacer>
									<v-btn @click="show('profile')" flat color="blue">BACK</v-btn>
									<v-btn :disabled="!form.valid" @click="submit" flat color="success">submit</v-btn>
									<!-- <v-btn @click="show('input_output')" flat color="success">NEXT</v-btn> -->
									
								</v-card-actions>
							</div>		
									
						</div>

						<div class="tab-pane tab-pane-content fade" id="profile" role="tabpanel" aria-labelledby="profile-tab">

							<div class=" div-list-content" >
								<v-card style="padding:0 2rem 2rem 2rem;">

									<v-tabs
										fixed-tabs
										v-model="model_create"
										centered
									>
										<v-tab
										:href="`#tab-minio`"
										>
											MINIO
										</v-tab>
										<v-tab
										:href="`#tab-onedata`"
										>
											ONE DATA
										</v-tab>
										<v-tab
										:href="`#tab-s3`"
										>
											S3
										</v-tab>
									</v-tabs>

									<v-tabs-items v-model="model_create" >
										<v-tab-item  :value="`tab-onedata`">
											<v-container>
												
									
												<v-layout row style="padding:0px,10px;justify-content: center;">										
													<img src="../../img/logo_one_data.jpg" alt=""> 
												</v-layout>
												<br>
												<v-flex xs12 text-xs-center>
													<span v-show="showErrorOneData" style="color: #cc3300; font-size: 12px;"><b>To add a storage option you must complete all the information.</b></span>                   									
												</v-flex>
												<br>

												<v-layout row style="padding:0px,10px;" >										
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field
															v-model="onedata.id"
															:counter="200"		
															label="ID"																
														></v-text-field>
													</v-flex>									
												</v-layout>

												<v-layout row style="padding:0px,10px;" >										
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field
															v-model="onedata.oneprovider_host"
															:counter="200"		
															label="ONEPROVIDER HOST:"																
														></v-text-field>
													</v-flex>									
												</v-layout>

												<v-flex xs12 text-xs-center>
													<span v-show="envrequirehost" style="color: #cc3300; font-size: 12px;"><b>ONEPROVIDER HOST is required</b></span>                   									
												</v-flex>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="onedata.token"
															:append-icon="showOneDataToken ? 'visibility_off' : 'visibility'"
															:type="showOneDataToken ? 'text' : 'password'"
															:counter="200"
															label="ACCESS TOKEN:"
															@click:append="showOneDataToken = !showOneDataToken"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-flex xs12 text-xs-center>
													<span v-show="envrequiretoken" style="color: #cc3300; font-size: 12px;"><b>ACCESS TOKEN is required</b></span>                   									
												</v-flex>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="onedata.space"
															:counter="200"
															label="SPACE:"
														></v-text-field>
													</v-flex>										
												</v-layout>

												<v-flex xs12 sm6 offset-sm3 v-show="showOneData">
													<input type="file" id="onedata" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>ONEDATA</v-subheader>
														<v-list-tile
														v-for="(id,key) in ONEDATA_DICT"
														:key="key"
														avatar
														@click.stop=""
														style="margin-bottom:40px;"
														>                                    
																

																<v-list-tile-content style="height:80px;">
																	<v-list-tile-title style="padding-bottom:20px;">ID: {{key}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">ONEPROVIDER HOST: {{id.oneprovider_host}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">ACCES TOKEN: <span class="hide_text">*********</span> </v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">SPACE: {{id.space}}</v-list-tile-title>
																	<!-- <v-list-tile-title>{{key}}</v-list-tile-title> -->
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeOneData(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>

												<v-card-actions class="text-md-center" >
													<v-spacer></v-spacer>
													<v-btn @click="includeOneData()"  color="info">ADD</v-btn>
													
												</v-card-actions>


											</v-container>	
										</v-tab-item>

										<v-tab-item  :value="`tab-minio`">
											<v-container>
												
									
												<v-layout row style="padding:0px,10px;justify-content: center;">										
													<img src="../../img/minio-storage.png" height="110px" alt=""> 
												</v-layout>
												<br>
												<v-flex xs12 text-xs-center>
													<span v-show="showErrorMinio" style="color: #cc3300; font-size: 12px;"><b>To add a storage option you must complete all the information.</b></span>                   									
												</v-flex>
												<br>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="minio.id"
															:counter="200"
															label="ID"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="minio.endpoint"
															:counter="200"
															label="ENDPOINT"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="minio.region"
															:counter="200"
															label="REGION"
														></v-text-field>
													</v-flex>										
												</v-layout>
												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="minio.secret_key"
															:append-icon="showMinioSecretKey ? 'visibility_off' : 'visibility'"
															:type="showMinioSecretKey ? 'text' : 'password'"
															:counter="200"
															label="SECRET KEY"
															@click:append="showMinioSecretKey = !showMinioSecretKey"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="minio.access_key"
															:append-icon="showMinioAccessKey ? 'visibility_off' : 'visibility'"
															:type="showMinioAccessKey ? 'text' : 'password'"
															:counter="200"
															label="ACCESS KEY"
															@click:append="showMinioAccessKey = !showMinioAccessKey"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex row xs12 sm8 offset-sm2>
															<span style="margin-top:16px;padding-top:4px;color:#605C5C;padding-right:10px;">VERIFY</span>
															<v-switch
																v-model="minio.verify"
															></v-switch>
													</v-flex>										
												</v-layout>

												<v-flex xs12 sm6 offset-sm3 v-show="showMinio">
													<input type="file" id="minio" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>MINIO</v-subheader>
														<v-list-tile
														v-for="(id,key) in MINIO_DICT"
														:key="key"
														avatar
														@click.stop=""
														style="margin-bottom:80px;"
														>                                    
																

																<v-list-tile-content style="height:120px;margin-top:40px;">
																	<v-list-tile-title style="padding-bottom:20px;">ID: {{key}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">ENDPOINT: {{id.endpoint}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">REGION: {{id.region}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">SECRET KEY: <span class="hide_text">*********</span></v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">ACCESS KEY: <span class="hide_text">*********</span></v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">VERIFY: {{id.verify}}</v-list-tile-title>
																	<!-- <v-list-tile-title>{{key}}</v-list-tile-title> -->
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeMinio(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>

												<v-card-actions class="text-md-center" >
													<v-spacer></v-spacer>
													<v-btn @click="includeMinio()"  color="info">ADD</v-btn>
													
												</v-card-actions>

											</v-container>
										</v-tab-item>

										<v-tab-item  :value="`tab-s3`">
											<v-container>
												
									
												<v-layout row style="padding:0px,10px;justify-content: center;">										
													<img src="../../img/amazon-s3.png" height="110px" alt=""> 
												</v-layout>
												<br>
												<v-flex xs12 text-xs-center>
													<span v-show="showErrorS3" style="color: #cc3300; font-size: 12px;"><b>To add a storage option you must complete all the information.</b></span>                   									
												</v-flex>
												<br>
												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="s3.id"
															:counter="200"
															label="ID"
														></v-text-field>
													</v-flex>										
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="s3.access_key"
															:append-icon="showS3AccessKey ? 'visibility_off' : 'visibility'"
															:type="showS3AccessKey ? 'text' : 'password'"
															:counter="200"
															label="ACCESS KEY"
															@click:append="showS3AccessKey = !showS3AccessKey"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="s3.secret_key"
															:append-icon="showS3SecretKey ? 'visibility_off' : 'visibility'"
															:type="showS3SecretKey ? 'text' : 'password'"
															:counter="200"
															label="SECRET KEY"
															@click:append="showS3SecretKey = !showS3SecretKey"
														></v-text-field>
													</v-flex>
												</v-layout>

												<v-flex xs12 text-xs-center>
													<span v-show="envrequiretoken" style="color: #cc3300; font-size: 12px;"><b>ACCESS TOKEN is required</b></span>                   									
												</v-flex>

												<v-layout row style="padding:0px,10px;">
													<v-flex xs12 sm8 offset-sm2>
														<v-text-field 
															v-model="s3.region"
															:counter="200"
															label="REGION"
														></v-text-field>
													</v-flex>										
												</v-layout>

												<v-flex xs12 sm6 offset-sm3 v-show="showS3">
													<input type="file" id="s3" hidden="true" multiple />
													<v-list subheader dense>
														<v-subheader inset>S3</v-subheader>
														<v-list-tile
														v-for="(id,key) in S3_DICT"
														:key="key"
														avatar
														@click.stop=""
														style="margin-bottom:40px;"
														>                                    
																

																<v-list-tile-content style="height:80px;">
																	<v-list-tile-title style="padding-bottom:20px;">ID: {{key}}</v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">ACCESS KEY: <span class="hide_text">*********</span></v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">SECRET TOKEN: <span class="hide_text">*********</span></v-list-tile-title>
																	<v-list-tile-title style="padding-bottom:20px;">REGION: {{id.region}}</v-list-tile-title>
																	<!-- <v-list-tile-title>{{key}}</v-list-tile-title> -->
																</v-list-tile-content>

																<v-list-tile-action>
																	<v-btn icon ripple @click="removeS3(key)">
																	<v-icon color="red lighten-1">remove_circle_outline</v-icon>
																	</v-btn>
																</v-list-tile-action>
														</v-list-tile>
													</v-list>
												</v-flex>

												<v-card-actions class="text-md-center" >
													<v-spacer></v-spacer>
													<v-btn @click="includeS3()"  color="info">ADD</v-btn>
													
												</v-card-actions>


											</v-container>
										</v-tab-item>
									</v-tabs-items>	
								</v-card>
							</div>
							<v-card-actions >
								<v-btn @click="closeWithoutSave()" flat color="grey">Cancel</v-btn>
								<v-btn @click="cleanfieldMinio();cleanfieldOneData();cleanfieldS3()" flat color="red">Clear</v-btn>
								<v-spacer></v-spacer>
								<v-btn  @click="show('home')" flat color="primary">BACK</v-btn>								
								<v-btn @click="show('input_output')" flat color="success">NEXT</v-btn>
								
							</v-card-actions>						
						</div>  
					</div>									
				</v-form>			
			</v-card>			
		</v-dialog>    
	</v-layout>
</template>

<script>
import axios from 'axios'
import Services from '../../components/services';
/* eslint-disable */
export default {   
	mixins:[Services],
	name: 'FunctionForm',
	data () {
		return {			
			dialog: false,        
			drawer: false,    
			url: "", 
			allText: "", 
			expand: "expand_more",
			editionMode: false,
			base64String : "",    
			filename: "",  
			files: [],    
			inputs:[],  
			prefixs_in:[],  
			suffixs_in:[],  
			outputs:[],  
			prefixs_out:[],  
			suffixs_out:[],  
			showUploading: false,
			showselectEnv: false,
			showselectInput: false,
			showselectPrefixIn: false,
			showselectSuffixIn: false,
			showselectOutput: false,
			showselectPrefixOut: false,
			showselectSuffixOut: false,
			showselectAnn: false,
			showOneDataToken: false,
			showS3AccessKey: false,
			showS3SecretKey: false,
			showMinioAccessKey: false,
			showMinioSecretKey: false,
			showOneData: false,
			showMinio: false,
			showS3: false,
			showErrorMinio:false,
			showErrorOneData:false,
			showErrorS3:false,
			showErrorInput:false,
			showErrorOutput:false,
			filerequire : false,
			envrequirehost: false,
			envrequiretoken : false,
			envrequirespace : false,
			envVars:{},
			envVarsAll:{},
			limits_mem: '',
			request_mem: '',
			select_logLevel: 'INFO',
			ONEDATA_DICT:{},
			S3_DICT:{},
			MINIO_DICT:{},
			 minio:{
				id:'',
				endpoint: '' ,
				region: '',
				secret_key: '',
				access_key: '',
				verify: true
			},
			s3:{
				id:'',
				access_key: '',
				secret_key: '',
				region: ''
			},
			onedata:{
				id:'',
				oneprovider_host: '',		
				token: '',		
				space: ''
			},

			form: {
				valid: false,
				image: '',
				imageRules: [
				v => !!v || 'Docker image is required',
				// v => (v && v.includes('/')) || 'The Docker image must comply with the nomenclature of Docker Hub images'
				// && /.+:.+/.test(v)
				],
				name: '',
				nameRules: [
				v => !!v || 'Function name is required'
				],
				envVarskey: "",
				envVarsValue: "",	
				path_in:"",
				log_level:['CRITICAL','ERROR','WARNING','INFO','DEBUG','NOTSET'],
				storage_provider_in:"",
				prefix_in:"",
				suffix_in:"",
				path_out:"",
				storage_provider_out:"",
				prefix_out:"",
				suffix_out:"",
				storage_provider:{
					// s3:{
					// 	access_key: '',
					// 	secret_key: '',
					// 	region: ''
					// },
					// onedata:{
					// 	oneprovider_host: '',		
					// 	token: '',		
					// 	space: ''
					// },
				},
				limits_cpu: '',
				limits_memory: '',
				regAuth: '',
				request_cpu: '',
				request_memory: '',
				secrets: '',
				script: '',
				
				
			},
			progress: {
				active: false
			},
			model_create: 'tab-minio',
			tabs_in_out: 'tab-input',
			showinput: true,
			memory: '',
			varsEnv: '',
			editScript: false,
			storages_all:[],
			select_tab:''


		}
	},
	watch:{
		"select_tab"(val){
			if(val == 'input_output'){				
					this.storages_all.push("minio.default")				
			}

		}
	},
	methods: {
		editSummernote(){
			var _this = this
            $('.summernote').summernote(
                {
					callbacks:{
						onInit: function() {
							 $('.summernote').summernote('codeview.activate');
						},		
					},
					codeviewFilter: true,
  					codeviewIframeFilter: true,
					focus: true,
					height: 200,                 // set editor height
					minHeight: null,             // set minimum height of editor
					maxHeight: null,             // set maximum height of editor
                    toolbar: [
                        // [groupName, [list of button]]
                        ['style', ['bold', 'italic', 'underline', 'clear']],
                        ['font', ['strikethrough', 'superscript', 'subscript']],
                        ['fontsize', ['fontsize']],
                        ['color', ['color']],
                        ['para', ['ul', 'ol', 'paragraph']],
						['height', ['height']],
						['view', ['codeview']]
					],
					codemirror: { // codemirror options
						theme: 'monokai',
						lineNumbers: true,
						lineWrapping: true,
    					tabMode: 'indent'
					},
					
                })
                .on("summernote.enter", function(we, e) {
                    $(this).summernote("pasteHTML", "<br><br>");
                    e.preventDefault();
				});
				 $('.summernote').summernote('code',_this.base64String)
        },
        saveSummernote(){
			this.base64String = $('.summernote').summernote('code')
			$('.summernote').summernote('destroy');
			setTimeout(function(){
				$('.summernote').css('display','none');
			},100)
            this.editScript = false
        },

		show(id){
			$("#myTabContent .tab-pane-content").removeClass("show active")
			$("#myTab .nav-link").removeClass("show active")
			$("#"+id).addClass("show active")
			$("#"+id+"-tab").addClass("show active")
			this.select_tab = id
		},
		show_input(id){
			$("#myTabContentInOut .tab-pane-inout").removeClass("show active")
			$("#myTabInOut .nav-link").removeClass("show active")
			$("#"+id).addClass("show active")
			$("#"+id+"-tab").addClass("show active")
		},
		
		handleUpdate(){
			this.$emit("SHOWSPINNER",true)			 
      		window.getApp.$emit('REFRESH_BUCKETS_LIST')
      		window.getApp.$emit('BUCKETS_REFRESH_DASHBOARD')
      		window.getApp.$emit('FUNC_GET_FUNCTIONS_LIST')
    	},
		cleanfield(){
			this.url=""
		},
		cleanfieldenv(){
			this.form.envVarskey=""
			this.form.envVarsValue=""
		},		
		cleanfieldInput(){
			this.form.path_in=""		
			this.form.storage_provider_in=""	
			this.cleanfieldPrefixIn()	
			this.cleanfieldSuffixIn()	
		},
		cleanfieldPrefixIn(){
			this.form.prefix_in=""		
		},
		cleanfieldSuffixIn(){
			this.form.suffix_in=""		
		},
		includePrefixIn(){
			if(this.form.prefix_in != null && this.form.prefix_in != ''){
				this.showselectPrefixIn = true
				this.prefixs_in.push(this.form.prefix_in)
				this.cleanfieldPrefixIn()
			}
		},
		includeSuffixIn(){
			if(this.form.suffix_in != null && this.form.suffix_in != ''){
				this.showselectSuffixIn = true
				this.suffixs_in.push(this.form.suffix_in)
				this.cleanfieldSuffixIn()
			}
		},
		includeInput(){
			if(this.form.storage_provider_in=='' || this.form.path_in==''){
				this.showErrorInput = true
			}else{
				this.showErrorInput = false
				this.showselectInput=true
				var input = {
					"path":this.form.path_in,
					"storage_provider":this.form.storage_provider_in,
					"prefix":this.prefixs_in,
					"suffix":this.suffixs_in
				}
				this.inputs.push(input)
				input = {}
				this.prefixs_in=[]
				this.suffixs_in=[]
				this.cleanfieldInput()						
				this.cleanfieldPrefixIn()						
				this.cleanfieldSuffixIn()	
				if (this.isEmpty(this.prefixs_in)) {
					this.showselectPrefixIn = false
				}
				if (this.isEmpty(this.suffixs_in)) {
					this.showselectSuffixIn = false
				}					
			}
		},
		cleanfieldOutput(){
			this.form.path_out=""		
			this.form.storage_provider_out=""	
			this.cleanfieldPrefixOut()	
			this.cleanfieldSuffixOut()	
		},
		cleanfieldPrefixOut(){
			this.form.prefix_out=""		
		},
		cleanfieldSuffixOut(){
			this.form.suffix_out=""		
		},
		includePrefixOut(){
			if(this.form.prefix_out != null && this.form.prefix_out != ''){
				this.showselectPrefixOut = true
				this.prefixs_out.push(this.form.prefix_out)
				this.cleanfieldPrefixOut()
			}
		},
		includeSuffixOut(){
			if(this.form.suffix_out != null && this.form.suffix_out != ''){
				this.showselectSuffixOut = true
				this.suffixs_out.push(this.form.suffix_out)
				this.cleanfieldSuffixOut()
			}
		},
		includeOutput(){
			if(this.form.storage_provider_out=='' || this.form.path_out==''){
				this.showErrorOutput = true
			}else{
				this.showErrorOutput = false
				this.showselectOutput=true
				var output = {
					"path":this.form.path_out,
					"storage_provider":this.form.storage_provider_out,
					"prefix":this.prefixs_out,
					"suffix":this.suffixs_out
				}
				this.outputs.push(output)
				output = {}
				this.prefixs_out=[]
				this.suffixs_out=[]
				this.cleanfieldOutput()						
				this.cleanfieldPrefixOut()						
				this.cleanfieldSuffixOut()	
				if (this.isEmpty(this.prefixs_out)) {
					this.showselectPrefixOut = false
				}
				if (this.isEmpty(this.suffixs_out)) {
					this.showselectSuffixOut = false
				}	
			}				
		},
		includeEnv(){
			if(this.form.envVarskey != null && this.form.envVarskey != '' && this.form.envVarsValue != null && this.form.envVarsValue != ''){
				this.showselectEnv=true
				var key= this.form.envVarskey.replace(" ", "")
				var value = this.form.envVarsValue.replace(" ", "")
				this.envVars[key]=value
				this.cleanfieldenv()						
			}
		},		
		includeOneData(){
			if(this.onedata.id=='' || this.onedata.oneprovider_host=='' || this.onedata.token=='' || this.onedata.space==''){
				this.showErrorOneData = true
				}else{
				this.showErrorOneData = false
				this.showOneData = true;
				var value_onedata = {
					"oneprovider_host": this.onedata.oneprovider_host,
					"token": this.onedata.token,
					"space": this.onedata.space
				}
				this.ONEDATA_DICT[this.onedata.id]=value_onedata;
				if (this.isEmpty(this.ONEDATA_DICT)) {
					this.showOneData = false
				}
				this.storages_all.push('onedata.'+this.onedata.id)
				value_onedata = ''
				this.cleanfieldOneData()
			}
			
		},
		cleanfieldOneData(){
			this.onedata.id=''
			this.onedata.oneprovider_host = ''
			this.onedata.space = ''
			this.onedata.token = ''
		},
		removeOneData(item){
			this.$delete(this.ONEDATA_DICT,item)			
			if (this.isEmpty(this.ONEDATA_DICT)) {
				this.showOneData = false
			}	

		},
		includeMinio(){
			if(this.minio.id=='' || this.minio.endpoint=='' || this.minio.region=='' || this.minio.secret_key=='' || this.minio.access_key==''){
				this.showErrorMinio = true
			}else{
				this.showErrorMinio = false
				this.showMinio = true;
				var value_minio = {
					"endpoint": this.minio.endpoint,
					"region": this.minio.region,
					"secret_key": this.minio.secret_key,
					"access_key": this.minio.access_key,
					"verify": this.minio.verify
				}
				this.MINIO_DICT[this.minio.id]=value_minio;
				if (this.isEmpty(this.MINIO_DICT)) {
					this.showMinio = false
				}
				
				this.storages_all.push("minio."+this.minio.id)
				value_minio = ''
				this.cleanfieldMinio()
			}
			
		},
		cleanfieldMinio(){
			this.minio.id=''
			this.minio.endpoint = ''
			this.minio.region = ''
			this.minio.secret_key = ''
			this.minio.access_key = ''
			this.minio.verify = true
		},
		removeMinio(item){
			this.$delete(this.MINIO_DICT,item)			
			if (this.isEmpty(this.MINIO_DICT)) {
				this.showMinio = false
			}	

		},
		includeS3(){
			if(this.s3.id=='' || this.s3.region=='' || this.s3.secret_key=='' || this.s3.access_key==''){
				this.showErrorS3 = true
			}else{
				this.showErrorS3 = false
				this.showS3 = true;
				var value_s3 = {
					"access_key": this.s3.access_key,
					"secret_key": this.s3.secret_key,
					"region": this.s3.region
				}
				this.S3_DICT[this.s3.id]=value_s3;
				if (this.isEmpty(this.S3_DICT)) {
					this.showS3 = false
				}
				this.storages_all.push("s3."+this.s3.id)
				value_s3 = ''
				this.cleanfieldS3()
			}
			
		},
		cleanfieldS3(){
			this.s3.id=''
			this.s3.access_key = ''
			this.s3.secret_key = ''
			this.s3.region = ''
		},
		removeS3(item){
			this.$delete(this.S3_DICT,item)			
			if (this.isEmpty(this.S3_DICT)) {
				this.showS3 = false
			}	

		},
		isEmpty(obj) {
			for(var key in obj) {
				if(obj.hasOwnProperty(key))
					return false;
			}
			return true;
		},
		removeEnv (key) {     
			this.$delete(this.envVars,key)			
			if (this.isEmpty(this.envVars)) {
				this.showselectEnv = false
			}		
		},	
		removeInput (key) {     
			this.$delete(this.inputs,key)			
			if (this.isEmpty(this.inputs)) {
				this.showselectInput = false
			}		
		},
		removeOutput (key) {     
			this.$delete(this.outputs,key)			
			if (this.isEmpty(this.outputs)) {
				this.showselectOutput = false
			}		
		},
		removePrefixIn (key) {     
			this.$delete(this.prefixs_in,key)			
			if (this.isEmpty(this.prefixs_in)) {
				this.showselectPrefixIn = false
			}		
		},
		removePrefixOut (key) {     
			this.$delete(this.suffixs_out,key)			
			if (this.isEmpty(this.suffixs_in)) {
				this.showselectPrefixOut = false
			}		
		},
		removeSuffixIn (key) {     
			this.$delete(this.suffixs_in,key)			
			if (this.isEmpty(this.suffixs_in)) {
				this.showselectSuffixIn = false
			}		
		},	
		removeSuffixOut (key) {     
			this.$delete(this.suffixs_out,key)			
			if (this.isEmpty(this.suffixs_out)) {
				this.showselectSuffixOut = false
			}		
		},	
		collapse(){			
			this.drawer = (!this.drawer)			
			if (this.drawer == true){
				this.expand = "expand_less"
				$("#panel").slideDown("slow");

			}else{
				this.expand = "expand_more"
				$("#panel").slideUp("slow");
			}			
		},
		addFiles () {      		
			this.$refs.files.click()		
		},
		removeFile (key) {     
			this.files.splice(key, 1)
			this.$refs.files.value = null
			// this.base64String = ''
			$('.summernote').summernote('destroy');
			setTimeout(function(){
				$('.summernote').css('display','none');
			},100)
		},		
		handleFilesUpload () {
			this.files = []      
			let uploadedFiles = this.$refs.files.files			
			this.filerequire = false
			/*
				Adds the uploaded file to the files array
			*/
			for (let i = 0; i < uploadedFiles.length; i++) {
				this.showUploading = false
				this.filename = uploadedFiles[i].name
				this.files.push(uploadedFiles[i])
			}			
			if (window.File && window.FileReader && window.FileList && window.Blob) {
				var f = this.files[0]; // FileList object
				
				var reader = new FileReader();
				// Closure to capture the file information.
				var _this = this;
				reader.onload = (function(theFile) {
				return function(e) {
					var binaryData = e.target.result;

					//Reading as String
					_this.base64String = binaryData
					//Converting Binary Data to base 64
					// _this.base64String = window.btoa(binaryData);
				};
				})(f);
				// Read in the image file as a data URL.
				// reader.readAsBinaryString(f);
				// reader.readAsBinaryString(f);
				reader.readAsText(f)
			} else {
				alert('The File APIs are not fully supported in this browser.');
			}      
		
		},
		readurl(){     
			if(this.url != '' && this.url!=null){
				this.files = [] 		
				let uploadedFiles = this.url						
				this.showUploading = false
				this.filename = this.url
				this.files.push(uploadedFiles)
				var _this = this
				fetch(this.url).then(r => r.blob()).then(blob => {
					var reader = new FileReader();
					reader.onload = function() {
						_this.base64String = reader.result.replace(/^data:.+;base64,/, '');
						// Convert to string because new OSCAR version doesn't need base64
						_this.base64String = atob(_this.base64String)
					};
					reader.readAsDataURL(blob);
				});
				this.cleanfield()
			}
		},
		closeWithoutSave() {      
			this.progress.active = false
			this.dialog = false            
			this.clear()      
		},
		extend(obj, src) {
				for (var key in src) {
					if (src.hasOwnProperty(key)) obj[key] = src[key];
				}
				return obj;
		},
		submit () {	

			if(this.$refs.form.validate() && this.editionMode == true && this.base64String != ''){
					this.editFunction()
			}else if (this.$refs.form.validate() && this.files.length != 0) {
				this.progress.active = true
				this.editionMode ? this.editFunction() : this.newFunction()				
				this.envrequirehost = false
				this.envrequiretoken = false
				this.envrequirespace = false
			}else {
				if (this.files.length == 0){
					this.filerequire = true
				}else{
					this.filerequire = false
				}	
				this.show('home')
			}
						
					
						
		},
		clearHome(){
			this.files = []
			this.url = ""
			this.$refs.files.value = null
			this.showUploading = false
			this.showselectEnv = false
			this.envVars = {}
			this.expand = "expand_more"
			$("#panel").slideUp("slow");
			$("#home-tab").addClass("show active")
			$("#home").addClass("show active")
			$("#input_output-tab").removeClass("show active")
			$("#input_output").removeClass("show active")
			$("#profile-tab").removeClass("show active")
			$("#profile").removeClass("show active")
			this.select_logLevel = 'INFO'
			this.editScript = false
			this.form.limits_cpu=''
			this.form.limits_memory=''
			this.form.image=''
			this.form.name=''
		},
		clear () {
			this.files = []
			this.url = ""
			this.$refs.files.value = null
			this.$refs.form.reset()
			this.editionMode = false
			this.showUploading = false
			this.showselectEnv = false
			this.envVars = {}
			this.anns = {}
			this.labels = {}
			this.expand = "expand_more"
			$("#panel").slideUp("slow");
			$("#home-tab").addClass("show active")
			$("#home").addClass("show active")
			$("#input_output-tab").removeClass("show active")
			$("#input_output").removeClass("show active")
			$("#profile-tab").removeClass("show active")
			$("#profile").removeClass("show active")
			this.minio.access_key = ''
			this.minio.secret_key = ''
			this.minio.endpoint = ''
			this.minio.verify = true
			this.onedata.oneprovider_host = ''
			this.onedata.token = ''
			this.onedata.space = ''
			this.s3.access_key = ''
			this.s3.secret_key = ''
			this.s3.region = ''
			this.prefixs_in = []
			this.suffixs_in = []
			this.inputs = []
			this.cleanfieldInput()
			this.prefixs_out = []
			this.suffixs_out = []
			this.outputs = []
			this.cleanfieldOutput()
			this.cleanfieldOneData()
			this.cleanfieldMinio()
			this.cleanfieldS3()
			this.showselectPrefixIn = false
			this.showselectSuffixIn = false
			this.showselectPrefixOut = false
			this.showselectSuffixOut = false
			this.showselectInput = false
			this.showselectOutput = false
			this.select_logLevel = 'INFO'
			this.editScript = false
			this.storages_all = []
			this.ONEDATA_DICT={}
			this.MINIO_DICT={}
			this.S3_DICT={}
			this.showOneData=false
			this.showMinio=false
			this.showS3=false
			this.base64String = ''
			$('.summernote').summernote('destroy');
			setTimeout(function(){
				$('.summernote').css('display','none');
			},100)
			this.show_input('input')
			
		},
		newFunction () {
			if (this.isEmpty(this.MINIO_DICT)==false) {
				this.form.storage_provider["minio"]=this.MINIO_DICT
			}
			if (this.isEmpty(this.S3_DICT)==false){
				this.form.storage_provider["s3"]=this.S3_DICT
			}
			if (this.isEmpty(this.ONEDATA_DICT)==false){
				this.form.storage_provider["onedata"]=this.ONEDATA_DICT
			}
			
			var value = $("#classmemory option:selected").text();			
				

			if (this.form.limits_memory == ""){
				this.limits_mem = ''
			}else{
				this.limits_mem = this.form.limits_memory + value;
			}

			var params = {
				
				'name': this.form.name, 
				'image': this.form.image, 
				'cpu': this.form.limits_cpu,
				'memory': this.limits_mem,
				'log_level': this.select_logLevel,
				'environment': {
					"Variables":this.envVars
				},
				'input': this.inputs,
				'output': this.outputs,
				'script': this.base64String,
				'storage_providers':this.form.storage_provider

			}
			this.createServiceCall(params,this.createServiceCallBack)	
			
		},
		createServiceCallBack(response){
			if(response.status == 201){
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `Function ${this.form.name} was successfully created.`, color: 'success', timeout: 12000 })
				this.dialog = false;
				this.clear()
				this.updateFunctionsGrid()
				window.getApp.$emit('REFRESH_BUCKETS_LIST')

			}else {
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
			}
			this.progress.active = false

		},
		editFunction () {
			if (this.isEmpty(this.MINIO_DICT)==false) {
				this.form.storage_provider["minio"]=this.MINIO_DICT
			}
			if (this.isEmpty(this.S3_DICT)==false){
				this.form.storage_provider["s3"]=this.S3_DICT
			}
			if (this.isEmpty(this.ONEDATA_DICT)==false){
				this.form.storage_provider["onedata"]=this.ONEDATA_DICT
			}
			
			var value = $("#classmemory option:selected").text();
			if (this.form.limits_memory == ""){
				this.limits_mem = ''
			}else{
				this.limits_mem = this.form.limits_memory + value;
			}
			var script = ''
			if (this.script == '') {
				script = this.base64String
			}else{
				script = this.script
			}
			var params = {
				
				'name': this.form.name, 
				'image': this.form.image, 
				'cpu': this.form.limits_cpu,
				'memory': this.limits_mem,
				'log_level': this.select_logLevel,
				'environment': {
					"Variables":this.envVars
				},
				'input': this.inputs,
				'output': this.outputs,
				'script': this.base64String,
				'storage_providers':this.form.storage_provider

			}	
			this.editServiceCall(params, this.editServiceCallBack)
			
		},
		editServiceCallBack(response){
			if(response.status == 204){
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: `Function ${this.form.name} has been updated`, color: 'success' })
				this.dialog = false;
				this.clear()
				this.updateFunctionsGrid()
				
			}else {
				window.getApp.$emit('APP_SHOW_SNACKBAR', { text: response, color: 'error' })
			}
			this.progress.active = false

		},
		updateFunctionsGrid () {
			window.getApp.$emit('FUNC_GET_FUNCTIONS_LIST')
			window.getApp.$emit('REFRESH_BUCKETS_LIST')
		}
	},
	computed: {
		
		formTitle () {
			return this.editionMode ? 'Edit Service' : 'New Service'
		},
		formColor () {
			return this.editionMode ? 'blue lighten-2' : 'teal lighten-3'
		},
		showSelectedFiles () {
			return this.files.length > 0
		},
		
	},
  
	created: function () {
		window.getApp.$on('FUNC_OPEN_MANAGEMENT_DIALOG', (data) => {
			this.dialog = true
			this.editionMode = data.editionMode
			this.form.name = data.name
			this.form.image = data.image
			this.inputs = data.input
			this.outputs = data.output
			this.form.limits_cpu = data.cpu
			var memory_split = []
			memory_split = data.memory.match(/[a-z]+|[^a-z]+/gi);
			this.form.limits_memory = memory_split[0]
			if (memory_split[1] == "Mi"){
				var value_select = "1"
			}else{
				var value_select = "2"
			}
			setTimeout(function(){
				$('#classmemory').val(value_select)
			},100)
			var key=''
			var values= ''
			key = Object.keys(data.envVars.Variables)
			values = Object.values(data.envVars.Variables)
			for (let i = 0; i < key.length; i++) {
				this.envVars[key[i]]=values[i]
			}
			if(key.length){
				this.showselectEnv = true
			}else{
				this.showselectEnv = true
			}
			setTimeout(function(){
				this.select_logLevel = data.log_Level
			},100)
			if (this.isEmpty(this.inputs)) {
				this.showselectInput = false
			}else{
				this.showselectInput = true
			}	
			if (this.isEmpty(this.outputs)) {
				this.showselectOutput = false
			}else{
				this.showselectOutput = true
			}
			this.base64String = data.script
			this.MINIO_DICT=data.storage_provider.minio
			this.ONEDATA_DICT=data.storage_provider.onedata
			this.S3_DICT=data.storage_provider.s3
			this.storages_all=[]
			if (this.isEmpty(this.MINIO_DICT)) {
				this.showMinio = false
			}else{
				this.showMinio = true
				this.storages_all.push('minio.'+Object.keys(this.MINIO_DICT))
			}
			if (this.isEmpty(this.ONEDATA_DICT)) {
				this.showOneData = false
			}else{
				this.showOneData = true
				this.storages_all.push('onedata.'+Object.keys(this.ONEDATA_DICT))
			}
			if (this.isEmpty(this.S3_DICT)) {
				this.S3_DICT = false
			}else{
				this.S3_DICT = true
				this.storages_all.push('s3.'+Object.keys(this.S3_DICT))

			}
		})
	}
}
</script>

<style scoped>

.hide_text{
	/* content:'*************'!important; */
	-webkit-text-security: disc!important;
	-moz-text-security: disc!important;
	/* text-security: disc; */
}

.active {
	color: black !important;
  	background-color: transparent !important;
  	border-bottom: yellowgreen solid 2px;
}
.list__tile {
  height: 6rem !important;
}
 #flip {
    /* padding: 5px; */
    /* text-align: center; */
    background-color: #e5eecc;
    border: solid 1px #c3c3c3;
}

#panel {
    /* padding: 50px; */
    display: none;
}

#test {
  text-align: left,
}
.v-btn.v-btn--outline {
    border: none;
    background: transparent !important;
    box-shadow: none;
}

	/* Small devices (landscape phones, 576px and up)*/
@media (min-width: 576px) { 
		.custom-select{
		height: calc(2.25rem + 3px);
		width: 100%
	}
	}

/*Medium devices (tablets, 768px and up)*/
@media (min-width: 768px) { 
	.custom-select{
		height: calc(2.25rem + 3px);
		width: 100%
	}
	}

/*Large devices (desktops, 992px and up)*/
@media (min-width: 992px) { 
	.custom-select{
		height: calc(2.25rem + 3px);
		width: 100%
	}
	}

/* Extra large devices (large desktops, 1200px and up)*/
@media (min-width: 1200px) { 
	.custom-select{
		height: calc(2.25rem + 3px);
		width: 75%
	}
}
</style>
