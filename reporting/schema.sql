create table apiguard_proxy_monitoring (
  "time" timestamp with time zone NOT NULL,
  service TEXT,
  proc_time float,
  status int,
  is_cached boolean NOT NULL DEFAULT false,
);
select create_hypertable('apiguard_proxy_monitoring', 'time');

create table apiguard_telemetry_monitoring (
  "time" timestamp with time zone NOT NULL,
  session_id TEXT,
  client_ip TEXT,
  MAIN_TILE_DATA_LOADED float,
  MAIN_TILE_PARTIAL_DATA_LOADED float,
  MAIN_SET_TILE_RENDER_SIZE float,
  score float
);
select create_hypertable('apiguard_telemetry_monitoring', 'time');

create table apiguard_backend_monitoring (
  "time" timestamp with time zone NOT NULL,
  service TEXT,
  is_cached boolean,
  action_type TEXT,
  proc_time float,
  indirect_call boolean
);
select create_hypertable('apiguard_backend_monitoring', 'time');

create table apiguard_alarm_monitoring (
  "time" timestamp with time zone NOT NULL,
  service TEXT,
  num_users int,
  num_requests int
);
select create_hypertable('apiguard_alarm_monitoring', 'time');