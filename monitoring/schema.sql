create table proxy_monitoring (
  "time" timestamp with time zone NOT NULL,
  service varchar(64),
  proc_time float,
  status int
);
select create_hypertable('proxy_monitoring', 'time');

create table telemetry_monitoring (
  "time" timestamp with time zone NOT NULL,
  session_id varchar(64),
  client_ip varchar(64),
  MAIN_TILE_DATA_LOADED float,
  MAIN_TILE_PARTIAL_DATA_LOADED float,
  MAIN_SET_TILE_RENDER_SIZE float,
  score float
);
select create_hypertable('telemetry_monitoring', 'time');

create table backend_monitoring (
  "time" timestamp with time zone NOT NULL,
  service varchar(64),
  is_cached boolean,
  action_type varchar(64),
  proc_time float,
  indirect_call boolean
);
select create_hypertable('backend_monitoring', 'time');

create table alarm_monitoring (
  "time" timestamp with time zone NOT NULL,
  service varchar(64),
  num_users int,
  num_requests int
);
select create_hypertable('alarm_monitoring', 'time');