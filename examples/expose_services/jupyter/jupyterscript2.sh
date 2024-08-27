sleep 10
start-notebook.sh --NotebookApp.allow_root=True  --Session.username=root  --NotebookApp.base_url=$JHUB_BASE_URL --NotebookApp.token=$JUPYTER_TOKEN  --NotebookApp.notebook_dir=$JUPYTER_DIRECTORY 
