"""Seed a temporary loopback Fleetglass server with deterministic cockpit fixtures."""
import json
import hashlib
import sys
from datetime import datetime, timedelta, timezone
from urllib.request import Request, urlopen

base = sys.argv[1].rstrip('/')
token = sys.argv[2]
now = datetime.now(timezone.utc)
def stamp(t): return t.isoformat().replace('+00:00', 'Z')
def probe(name, state, detail, **values): return {'name': name, 'state': state, 'detail': detail, 'values': values}
def post(batch):
    req = Request(base + '/api/v1/ingest', json.dumps(batch).encode(), {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'}, method='POST')
    with urlopen(req, timeout=10) as response:
        assert response.status == 204

post({'host_id':'test-host','at':stamp(now-timedelta(days=8)),'results':[],
      'storage':[{'root':'/srv/projects','path':'/srv/projects/atlas','depth':1,'size_bytes':700_000_000,'complete':True}]})

for hour in range(24, -1, -1):
    at = now - timedelta(hours=hour)
    cpu = [5, 7, 13, 8, 25, 10, 6, 19, 42, 12, 7, 9, 18, 39, 7, 5, 9, 32, 12, 8, 5, 6, 18, 14, 5][24-hour]
    results = [
        probe('cpu', 'ok', f'CPU busy {cpu}%', busy_percent=cpu, core_count=8, user_percent=cpu*.65, system_percent=cpu*.3, iowait_percent=cpu*.04, steal_percent=cpu*.01, **{f"core_{i}_percent": cpu*(.7+i*.08) for i in range(8)}),
        probe('load', 'ok', 'system load averages', **{'1m': 0.42, '5m': 0.68, '15m': 0.54}),
        probe('memory', 'ok', 'memory and swap usage', total_bytes=16_750_000_000, used_bytes=6_100_000_000, swap_total_bytes=2_000_000_000, swap_used_bytes=220_000_000, page_cache_bytes=1_900_000_000, buffers_bytes=300_000_000, shared_bytes=110_000_000, pressure_some_avg10=0.2, pressure_some_avg60=0.1, pressure_some_avg300=0.05),
        probe('disk:/', 'ok', 'filesystem usage', total_bytes=500_000_000_000, used_bytes=112_000_000_000, inodes_total=10_000_000, inodes_used=2_000_000),
        probe('drive:nvme0n1', 'ok', 'drive activity', read_bytes_per_second=2_000_000+cpu*20000, write_bytes_per_second=1_200_000+cpu*10000, written_today_bytes=9_000_000_000),
        probe('cpu_package_power', 'ok', 'CPU package power', watts=4 + cpu * .4, energy_kwh=.001, energy_seconds=300),
        probe('sensor:coretemp:Package id 0', 'ok', 'package temperature', celsius=33 + cpu * .2, max_celsius=80),
        probe('sensor:nct6798:CPU fan', 'ok', 'fan speed', rpm=800),
        probe('systemd:failed', 'ok', '0 failed units', count=0),
        probe('systemd:caddy.service', 'ok', 'active (running)'),
        probe('systemd:agent.service', 'ok', 'active (running)'),
        probe('docker:0', 'ok', '3 containers', count=3),
        probe('container:0:backend', 'ok', 'running; healthy', restart_count=0),
        probe('container:0:postgres', 'ok', 'running; healthy', restart_count=0),
        probe('container:0:worker', 'ok', 'running; healthy', restart_count=0),
        probe('network:eth0', 'ok', 'interface throughput', rx_bytes=50_000_000_000+(24-hour)*10_000_000, tx_bytes=15_000_000_000+(24-hour)*5_000_000, rx_bytes_per_second=2900+cpu*50, tx_bytes_per_second=1600+cpu*20),
        probe('network:lo', 'ok', 'interface throughput', rx_bytes=1_000_000, tx_bytes=1_000_000, rx_bytes_per_second=20, tx_bytes_per_second=20),
        probe('network:docker0', 'ok', 'interface throughput', rx_bytes=0, tx_bytes=0, rx_bytes_per_second=0, tx_bytes_per_second=0),
        probe('network:wlan0', 'ok', 'interface throughput', rx_bytes=0, tx_bytes=0, rx_bytes_per_second=0, tx_bytes_per_second=0),
        probe('network:tailscale0', 'ok', 'interface throughput', rx_bytes=15_000_000, tx_bytes=190_000_000, rx_bytes_per_second=40, tx_bytes_per_second=100),
        probe('tls:example.test', 'ok', '82 days until expiry', days=82),
        probe('tls:expiring.test', 'warn', 'expires in 9 days', days_remaining=9),
        probe('port:tcp:ipv4:443:any', 'ok', 'listening tcp port', port=443),
        probe('port:tcp:ipv4:7878:loopback', 'ok', 'listening tcp port', port=7878),
        probe('smart:0', 'ok', 'disk health', wear_percent=5, temperature_celsius=34),
    ]
    if hour == 0:
        results.extend([
            probe('sensor:cpu:Package', 'warn', 'package temperature high', celsius=85),
            {**probe('internal:closed_target','crit','target unreachable',latency_ms=2), 'fields': {'display_name': 'Closed target'}},
            probe('recent_errors', 'warn', 'journal, last 24 h', count=14, oom_count=1),
            {**probe('recent_error:0', 'warn', 'journal error group', count=11, last_unix_us=int(at.timestamp()*1_000_000)), 'fields': {'unit':'livepatch.service','message':'Refresh failed while checking metadata'}},
            {**probe('recent_error:1', 'warn', 'journal error group', count=3, last_unix_us=int(at.timestamp()*1_000_000)), 'fields': {'unit':'networkd.service','message':'Startup timed out'}},
            {**probe('oom:0', 'crit', 'kernel out-of-memory kill', count=1), 'fields': {'process':'python3'}},
            probe('recent_errors:container_restarts', 'ok', 'Docker restart count increase', count=0),
            probe('security:updates', 'warn', '8 pending, 2 security', total=8, security=2, reboot_required=0),
            {**probe('security:ssh_logins', 'ok', 'SSH logins, last 24 h', count=4), 'fields': {'last_user':'agent','source_class':'tailnet'}},
            probe('security:sudo', 'ok', 'sudo use, last 24 h', count=2),
            probe('security:failed_logins', 'warn', 'failed SSH logins, last 24 h', count=3),
            {**probe('security:failed_source:0', 'warn', 'failed login source class', count=3), 'fields': {'source_class':'public'}},
            probe('security:sensitive_files', 'ok', 'fixed sensitive file list', monitored=7, changes=0),
        ])
    batch = {'host_id': 'test-host', 'at': stamp(at), 'results': results}
    batch['probes'] = [{'name':'fixture_site','display_name':'Fixture site','kind':'http','state':'ok','interval_seconds':3600,'at':stamp(at),'latency_ms':42}]
    batch['apps'] = [
        {'name':'fleetglass-agent.service','kind':'unit','rates_ready':True,'counter_source':'cgroup','disk_read_counter_bytes':1_000_000_000+(24-hour)*10_000_000,'disk_write_counter_bytes':500_000_000+(24-hour)*5_000_000,'cpu_percent':cpu*.32,'memory_bytes':680_000_000,'page_cache_bytes':42_000_000,'disk_read_bytes_per_second':220_000,'disk_write_bytes_per_second':130_000,'disk_read_bytes':19_000_000,'disk_write_bytes':12_000_000,'estimated_energy_watts':2.3,'processes':[{'name':'fleetglass-agent','pid':101}]},
        {'name':'backend','kind':'container','rates_ready':True,'counter_source':'cgroup','disk_read_counter_bytes':700_000_000+(24-hour)*8_000_000,'disk_write_counter_bytes':400_000_000+(24-hour)*6_000_000,'cpu_percent':cpu*.24,'memory_bytes':1_100_000_000,'page_cache_bytes':120_000_000,'disk_read_bytes_per_second':190_000,'disk_write_bytes_per_second':210_000,'disk_read_bytes':22_000_000,'disk_write_bytes':31_000_000,'network_rx_bytes_per_second':1100,'network_tx_bytes_per_second':700,'estimated_energy_watts':1.8},
        {'name':'Codex session · fleetglass','kind':'agent','rates_ready':True,'counter_source':'proc','disk_read_counter_bytes':100_000_000+(24-hour)*6_000_000,'disk_write_counter_bytes':80_000_000+(24-hour)*3_000_000,'cpu_percent':cpu*.18,'memory_bytes':420_000_000,'page_cache_bytes':0,'disk_read_bytes_per_second':440_000,'disk_write_bytes_per_second':200_000,'disk_read_bytes':44_000_000,'disk_write_bytes':20_000_000,'estimated_energy_watts':1.2,'processes':[{'name':'codex','pid':1202}]},
    ]
    if hour == 0:
        batch['apps'].extend({'name':f'worker-{i:02d}.service','kind':'unit','rates_ready':True,'counter_source':'proc','cpu_percent':i/10,'memory_bytes':50_000_000+i*1_000_000,'page_cache_bytes':0,'disk_read_bytes_per_second':i*1000,'disk_write_bytes_per_second':i*500,'disk_read_bytes':0,'disk_write_bytes':0,'estimated_energy_watts':i/100,'processes':[{'name':'worker','pid':4000+i}]} for i in range(60))
        batch['apps'].append({'name':'runaway-build.service','kind':'unit','rates_ready':True,'cpu_percent':90,'memory_bytes':250_000_000})
        batch['apps'].append({'name':'vite.service','kind':'unit','rates_ready':True,'cpu_percent':0,'memory_bytes':310_000_000,'processes':[{'name':'vite','pid':9123}]})
        batch['projects']=[
            {'pid':9123,'name':'vite','project':'atlas','worktree':'dashboard','kind':'dev_server','cpu_percent':0,'memory_bytes':310_000_000,'ports':[5173],'ports_measured':True,'connections_measured':True,'idle_minutes':196,'connection_idle_minutes':196,'idle':True,'stop_command':'kill -TERM -- 9123'},
            {'pid':1202,'name':'codex','project':'fleetglass','worktree':'cockpit','kind':'agent','cpu_percent':2.4,'memory_bytes':319_000_000,'ports':[],'ports_measured':True,'connections_measured':True,'idle_minutes':0,'connection_idle_minutes':0,'idle':False,'stop_command':'kill -TERM -- 1202'},
        ]
        batch['storage']=[{'root':'/srv/projects','path':'/srv/projects/atlas','depth':1,'size_bytes':920_000_000,'complete':True},
                          {'root':'/srv/projects','path':'/srv/projects','depth':0,'size_bytes':1_400_000_000,'complete':True}]
        for minute in range(11,0,-1):
            post({'host_id':'test-host','at':stamp(now-timedelta(minutes=minute)),'results':[],
                  'apps':[{'name':'runaway-build.service','kind':'unit','rates_ready':True,'cpu_percent':90,'memory_bytes':250_000_000}]})
    if hour == 0:
        batch['changes'] = [{'id': hashlib.sha256(b'fixture-apt').hexdigest(), 'at': stamp(at), 'type': 'APT', 'text': 'Upgraded 2 packages: curl, git'}]
    batch['ai'] = {'usage': [{
        'id': hashlib.sha256(f'usage-hour-{hour}'.encode()).hexdigest(),
        'session': 'a'*64, 'kind': 'claude', 'model': 'claude-sonnet-5',
        'project': 'atlas', 'at': stamp(at),
        'tokens': {'input': 5000 + cpu*100, 'output': 1000, 'cache_write': 0, 'cache_read': 0},
        'source': 'otlp',
    }]}
    if hour == 0:
        session_a = 'a'*64
        session_b = 'b'*64
        batch['ai'].update({
            'sessions': [
                {'id':session_a,'kind':'claude','model':'claude-sonnet-5','project':'atlas','worktree':'dashboard','user':'agent','pid':1201,'cpu_percent':34,'rss_bytes':410_000_000,'at':stamp(at),'state':'working','reason':'recent activity','source':'otlp'},
                {'id':session_b,'kind':'codex','model':'gpt-6','project':'fleetglass','worktree':'cockpit','user':'agent','pid':1202,'cpu_percent':2.4,'rss_bytes':319_000_000,'at':stamp(at),'state':'waiting-for-input','reason':'last turn ended','source':'log'},
            ] + [
                {'id': hashlib.sha256(f'root-claude-{i}'.encode()).hexdigest(), 'kind':'claude', 'project':'coordination', 'user':'root', 'pid':2000+i, 'cpu_percent':1+i, 'rss_bytes':280_000_000+i*30_000_000, 'at':stamp(at), 'state':'running', 'reason':'process alive', 'source':'process'}
                for i in range(6)
            ] + [
                {'id': hashlib.sha256(f'agent-extra-{i}'.encode()).hexdigest(), 'kind':'codex', 'project':f'project-{i}', 'user':'agent', 'pid':3000+i, 'cpu_percent':i+1, 'rss_bytes':250_000_000, 'at':stamp(at), 'state':'running', 'reason':'process alive', 'source':'process'}
                for i in range(6)
            ],
            'quotas': [
                {'kind':'codex','window':'primary','window_minutes':10080,'used_percent':64,'resets_at':stamp(now+timedelta(days=5, hours=18)),'observed_at':stamp(now),'method':'cli_reported'},
                {'kind':'codex','window':'secondary','window_minutes':300,'used_percent':28,'resets_at':stamp(now+timedelta(hours=3)),'observed_at':stamp(now),'method':'cli_reported'},
            ],
        })
        batch['ai']['usage'].extend([
            {'id':'c'*64,'session':session_a,'kind':'claude','model':'claude-sonnet-5','project':'atlas','at':stamp(at),'tokens':{'input':1_200_000,'output':130_000,'cache_write':0,'cache_read':200_000},'source':'otlp'},
            {'id':'d'*64,'session':session_b,'kind':'codex','model':'gpt-6','project':'fleetglass','at':stamp(at),'tokens':{'input':900_000,'output':70_000,'cache_write':0,'cache_read':0},'source':'otlp'},
        ])
    post(batch)
print('Seeded test-host with 25 local samples.')
