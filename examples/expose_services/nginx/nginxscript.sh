echo "server {  
    listen 80;  
    server_name 0.0.0.0;

    location / {  
        default_type text/plain;
        return 200 'Welcome to nginx! Message: ${MESSAGE:-missing-env}';  
    } 
}" > /etc/nginx/conf.d/default.conf
nginx -g 'daemon off;' 
