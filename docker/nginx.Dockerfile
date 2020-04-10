FROM nginx:alpine

# set nignx timezone to UTC - but the server still vends local time. TODO: see whether this will work or not
ENV TZ UTC
# source customized config
COPY ./config/nginx.conf /etc/nginx/nginx.conf
