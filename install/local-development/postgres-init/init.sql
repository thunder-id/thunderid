-- Create databases
CREATE DATABASE runtime_transient;
CREATE DATABASE configdb;
CREATE DATABASE entitydb;
CREATE DATABASE runtime_persistent;

-- Run db1 initialization
\connect runtime_transient
\i /docker-entrypoint-initdb.d/runtime-transient-postgres.sql

-- Run db2 initialization
\connect configdb
\i /docker-entrypoint-initdb.d/config-postgres.sql

-- Run db3 initialization
\connect entitydb
\i /docker-entrypoint-initdb.d/entity-postgres.sql

-- Run db4 initialization
\connect runtime_persistent
\i /docker-entrypoint-initdb.d/runtime-persistent-postgres.sql
