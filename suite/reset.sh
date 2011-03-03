#!/bin/sh
rm -rf *.pem cert sniffy.db sniffy.log
dbstmt="""DROP DATABASE sniffy;
CREATE DATABASE sniffy
  WITH OWNER = sniffy
       ENCODING = 'UTF8'
       TABLESPACE = pg_default
       LC_COLLATE = 'en_US.UTF-8'
       LC_CTYPE = 'en_US.UTF-8'
       CONNECTION LIMIT = -1;"""
echo $dbstmt | psql postgres
