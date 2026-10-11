/** UI messages are keyed independently of their English wording. Observed host data never enters this catalog. */
export type Language = "en" | "fr";
export const unchangedTechnicalTerms = [
  "CPU", "MCP", "API", "GPU", "GPU %", "RAM", "DNS", "TLS", "HTTP", "PID", "· PID", "24h", "30d",
  "kWh", "ms", "p50", "p95", "°C", "°F", "USD", "SMART", "RSS", "sudo", "cache", "threads", "total", "port", "session", "APT", "Git", "Docker",
  "OpenTelemetry", "systemd · TLS", "Bits/s", "{used} / {total} inodes", "⌘K", "1 h", "24 h",
] as const;
export const unchangedFrenchCognates = [
  "Agent", "Agents", "Fleetglass", "Français", "Machine", "Quotas",
  "Services", "Sessions", "Type", "Volumes", "Navigation", "Sections",
  "Service", "Service {name}", "{count} sessions",
] as const;
export const messages = {
  "terminal.label_24h": ["24h", "24h"],
  "terminal.label_30d": ["30d", "30d"],
  "terminal.label_dns": ["DNS", "DNS"],
  "terminal.label_smart": ["SMART", "SMART"],
  "terminal.label_age": ["age", "âge"],
  "terminal.label_amount": ["amount", "montant"],
  "terminal.label_app": ["app", "application"],
  "terminal.label_available": ["available", "disponible"],
  "terminal.label_buffers": ["buffers", "tampons"],
  "terminal.label_cache": ["cache", "cache"],
  "terminal.label_certificate": ["certificate", "certificat"],
  "terminal.label_cpu": ["cpu", "CPU"],
  "terminal.label_days": ["days", "jours"],
  "terminal.label_detail": ["detail", "détail"],
  "terminal.label_disk_i_o": ["disk i/o", "E/S disque"],
  "terminal.label_energy": ["energy", "énergie"],
  "terminal.label_event": ["event", "événement"],
  "terminal.label_fans": ["fans", "ventilateurs"],
  "terminal.label_frequency": ["frequency", "fréquence"],
  "terminal.label_idle": ["idle", "inactif"],
  "terminal.label_iface": ["iface", "interface"],
  "terminal.label_iowait_steal": ["iowait/steal", "attente E/S / vol"],
  "terminal.label_kind": ["kind", "type"],
  "terminal.label_listening": ["listening", "en écoute"],
  "terminal.label_load_1_5_15": ["load 1/5/15", "charge 1/5/15"],
  "terminal.label_mem": ["mem", "mémoire"],
  "terminal.label_model": ["model", "modèle"],
  "terminal.label_mount": ["mount", "point de montage"],
  "terminal.label_name": ["name", "nom"],
  "terminal.label_package": ["package", "boîtier"],
  "terminal.label_per_core": ["per-core", "par cœur"],
  "terminal.theme": ["theme", "thème"],
  "terminal.label_source": ["source", "origine"],
  "terminal.label_plan": ["plan", "forfait"],
  "terminal.label_port": ["port", "port"],
  "terminal.label_pressure": ["pressure", "pression"],
  "terminal.label_probe": ["probe", "sonde"],
  "terminal.label_process": ["process", "processus"],
  "terminal.label_project": ["project", "projet"],
  "terminal.label_reboot": ["reboot", "redémarrage"],
  "terminal.label_reset": ["reset", "réinitialisation"],
  "terminal.label_rss": ["rss", "RSS"],
  "terminal.label_rx_s": ["rx/s", "réception/s"],
  "terminal.label_session": ["session", "session"],
  "terminal.label_set": ["set", "jeu"],
  "terminal.label_size": ["size", "taille"],
  "terminal.label_ssh_failed": ["ssh failed", "échecs SSH"],
  "terminal.label_state": ["state", "état"],
  "terminal.label_sudo": ["sudo", "sudo"],
  "terminal.label_swap": ["swap", "échange"],
  "terminal.label_temp": ["temp", "temp."],
  "terminal.label_threads": ["threads", "threads"],
  "terminal.label_time": ["time", "heure"],
  "terminal.label_total": ["total", "total"],
  "terminal.label_tx_s": ["tx/s", "envoi/s"],
  "terminal.label_unit": ["unit", "unité"],
  "terminal.label_updates": ["updates", "mises à jour"],
  "terminal.label_used": ["used", "utilisé"],
  "terminal.label_value": ["value", "valeur"],
  "terminal.label_in": ["in", "entrée"],
  "terminal.label_out": ["out", "sortie"],
  "terminal.label_cache_tokens": ["cache", "cache"],
  "terminal.label_tokens_second": ["tok/s", "jetons/s"],
  "terminal.label_tokens_minute": ["tok/min", "jetons/min"],
  "terminal.label_requests_minute": ["req/min", "requêtes/min"],
  "terminal.label_median": ["median", "médiane"],
  "terminal.label_p95": ["p95", "p95"],
  "terminal.label_errors": ["errors", "erreurs"],
  "terminal.label_retries": ["retries", "tentatives"],
  "terminal.label_page_cache": ["page cache", "cache de pages"],
  "terminal.label_user_sys": ["user/sys", "utilisateur/système"],
  "terminal.label_window": ["window", "fenêtre"],
  "terminal.not_collected": ["not collected", "non collecté"],
  "terminal.monthly": ["monthly", "mensuel"],
  "terminal.api_used": ["API used", "API utilisé"],
  "terminal.value_ratio": ["value ratio", "rapport valeur"],
  "terminal.speed": ["speed", "vitesse"],
  "terminal.pace": ["pace", "rythme"],
  "terminal.collected": ["collected", "collectés"],
  "terminal.top_memory": ["top by RSS", "premiers par RSS"],
  "terminal.attention": ["attention", "à surveiller"],
  "terminal.stands_out": ["what stands out", "faits marquants"],
  "terminal.no_insights": ["no insights reported", "aucune observation signalée"],
  "terminal.ai_agents": ["ai agents", "agents IA"],
  "terminal.active": ["active", "actifs"],
  "terminal.today": ["today", "aujourd’hui"],
  "terminal.top_apps": ["top apps", "premières applications"],
  "terminal.disks": ["disks", "disques"],
  "terminal.containers": ["containers", "conteneurs"],
  "terminal.backups": ["backups", "sauvegardes"],
  "terminal.security": ["security", "sécurité"],
  "terminal.events": ["events", "événements"],
  "terminal.range": ["range", "période"],
  "terminal.views": ["views", "vues"],
  "terminal.filter": ["filter", "filtrer"],
  "terminal.search": ["search", "rechercher"],
  "terminal.glossary": ["glossary", "glossaire"],
  "terminal.status_up": ["up", "actif depuis"],
  "terminal.status_kernel": ["kernel", "noyau"],
  "terminal.status_load": ["load", "charge"],
  "terminal.pace_marker": ["│ = pace", "│ = rythme"],
  "terminal.dev_servers": ["dev servers", "serveurs de développement"],
  "terminal.required": ["required", "nécessaire"],
  "terminal.not_required": ["not required", "non nécessaire"],
  "terminal.read_only": ["read-only", "lecture seule"],
  "terminal.restore_test": ["restore test", "test de restauration"],
  "terminal.offsite_copy": ["off-site copy", "copie distante"],
  "terminal.ssh_logins": ["ssh logins", "connexions SSH"],
  "terminal.glossary_intro": ["How to read the cockpit", "Comment lire le cockpit"],
  "terminal.glossary_bar": ["Solid blocks show used capacity; light blocks show remaining capacity. The vertical line marks steady pace.", "Les blocs pleins indiquent la capacité utilisée ; les blocs clairs indiquent la capacité restante. La barre verticale marque le rythme régulier."],
  "terminal.glossary_unknown": ["Not collected means this host has not supplied that measurement.", "Non collecté signifie que cet hôte n’a pas fourni cette mesure."],
  "terminal.glossary_warn": ["WARN marks a condition that needs review.", "WARN signale une situation à examiner."],
  "terminal.glossary_crit": ["CRIT marks a critical condition.", "CRIT signale une situation critique."],
  "release.version": ["Version {version}", "Version installée {version}"],
  "release.update": ["Version {version} is available", "La version {version} est disponible"],
  "worktree.title": ["Worktrees", "Arbres de travail"],
  "worktree.age_label": ["Age", "Âge"],
  "worktree.last_commit": ["Last commit", "Dernier commit"],
  "worktree.merge_status": ["Merge", "Fusion"],
  "worktree.reclaimable": ["{size} in merged idle worktrees could be reclaimed", "{size} pourraient être récupérés dans les arbres fusionnés et inactifs"],
  "worktree.incomplete": ["Size incomplete", "Taille incomplète"],
  "worktree.merge_unknown": ["Merge unknown", "Fusion inconnue"],
  "worktree.merged": ["Merged", "Fusionné"],
  "worktree.unmerged": ["Not merged", "Non fusionné"],
  "worktree.stale": ["Stale by the 72 h rule", "Ancien selon la règle des 72 h"],
  "worktree.reclaimable_tag": ["Reclaimable", "Récupérable"],
  "session.title": ["{agent} session", "Session {agent}"],
  "session.start": ["Start", "Début"],
  "session.end": ["End", "Fin"],
  "session.running": ["Running", "En cours"],
  "session.duration": ["Duration", "Durée"],
  "session.spawned": ["Spawned processes", "Processus lancés"],
  "session.no_spawned": ["No spawned processes observed", "Aucun processus lancé observé"],
  "project.left_behind": ["Left behind by agents", "Laissés par les agents"],
  "project.witnessed_ancestry": ["Witnessed process tree", "Arbre de processus observé"],
  "project.freeable": ["Estimated freeable process RSS: {memory}", "Mémoire RSS potentiellement libérable (estimation) : {memory}"],
  "project.copy_only": ["Review the process before copying a stop command. Fleetglass takes no action.", "Examinez le processus avant de copier une commande d’arrêt. Fleetglass n’agit pas."],
  "subscription.title": ["Subscription versus API", "Abonnement face à l’API"],
  "subscription.this_month": ["Current calendar month", "Mois en cours"],
  "subscription.api_equivalent": ["API equivalent at configured model prices", "Équivalent API aux tarifs des modèles configurés"],
  "subscription.no_config_foot": ["API equivalent is an estimate, not a bill.", "L’équivalent API est une estimation, pas une facture."],
  "speed.title": ["Speed", "Vitesse"],
  "speed.one_hour": ["1 h", "1 h"],
  "speed.day": ["24 h", "24 h"],
  "speed.last_hour": ["Last hour", "Dernière heure"],
  "speed.no_data": ["No speed measurements yet", "Aucune mesure de vitesse pour le moment"],
  "pwa.offline": ["Offline", "Hors connexion"],
  "pwa.last_data": ["Last data at {at}", "Dernières données le {at}"],
  "result.certificate_expires": ["Expires in {days} days", "Expire dans {days} jours"],
  "result.certificate_expired": ["Expired {days} days ago", "Expiré depuis {days} jours"],
  "result.active": ["active", "actif"],
  "result.healthy": ["healthy", "sain"],
  "result.running": ["running", "en cours"],
  "result.unhealthy": ["unhealthy", "défaillant"],
  "result.starting": ["starting", "en démarrage"],
  "result.systemd_active": ["active ({substate})", "actif ({substate})"],
  "result.container_status": ["{state}; {health}", "{state} ; {health}"],
  "result.observed": ["Observed", "Observé"],
  "result.configured_status_value": ["Configured status value", "Valeur d’état configurée"],
  "result.recent_timestamp": ["Recent timestamp", "Horodatage récent"],
  "result.log_entry_observed": ["Matching log entry observed", "Entrée de journal correspondante observée"],
  "result.tcp_connect_succeeded": ["TCP connect succeeded", "Connexion TCP réussie"],
  "change.apt_upgraded": ["Upgraded {count} packages: {packages}", "{count} paquets mis à jour : {packages}"],
  "change.apt_upgraded_one": ["Upgraded {count} package: {packages}", "{count} paquet mis à jour : {packages}"],
  "change.service_restarted": ["{name} restarted", "{name} redémarré"],
  "change.service_failed": ["{name} failed", "{name} en échec"],
  "change.machine_restarted": ["Machine restarted", "Machine redémarrée"],
  "change.container_image_changed": ["Image changed: {name}", "Image modifiée : {name}"],
  "change.marker_name": ["{name}", "{name}"],
  "change.local_history": ["{count} events · local history", "{count} événements · historique local"],
  "cost.cli_reported": ["Reported by CLI", "Signalé par le CLI"],
  "cost.subscription_allocated": ["Subscription allocated", "Abonnement réparti"],
  "cost.partial": ["Partial · {tokens} unpriced tokens", "Partiel · {tokens} jetons sans tarif"],
  "explain.cpu": ["CPU use is measured across the whole machine. An app can use several cores, so its one-core percentage may exceed 100%.","L’usage du processeur couvre toute la machine. Une application peut utiliser plusieurs cœurs : son pourcentage rapporté à un cœur peut dépasser 100 %."],
  "explain.memory": ["Used memory includes application data and reclaimable cache. Page cache keeps recently read files in RAM and can be released when needed.","La mémoire utilisée comprend les applications et le cache récupérable. Le cache des pages garde en RAM les fichiers lus récemment et peut être libéré au besoin."],
  "explain.swap": ["Swap moves less-used memory pages out of RAM. Some swap use is normal; sustained swapping can slow work.","La mémoire d’échange déplace hors de la RAM les pages peu utilisées. Une petite quantité est normale ; une activité soutenue peut ralentir la machine."],
  "explain.load": ["The average number of tasks running or waiting for CPU or disk, over 1, 5 and 15 minutes. Compare it with the number of CPU cores.","Nombre moyen de tâches en cours ou en attente du processeur ou du disque, sur 1, 5 et 15 minutes. À comparer au nombre de cœurs."],
  "explain.iowait": ["Time when the CPU is idle while a task waits for disk I/O. It does not mean the CPU itself is busy.","Temps pendant lequel le processeur est inactif alors qu’une tâche attend le disque. Le processeur lui-même n’est pas occupé."],
  "explain.steal": ["CPU time taken by the hypervisor from this virtual machine. It is usually zero on a physical host.","Temps processeur retiré à cette machine virtuelle par l’hyperviseur. Il est normalement nul sur un serveur physique."],
  "explain.page_cache": ["RAM used to keep file contents ready for reuse. Linux can reclaim it for applications.","RAM qui conserve les fichiers récemment lus. Linux peut la récupérer pour les applications."],
  "explain.zram": ["Compressed swap stored in RAM. It saves space at the cost of some CPU time.","Mémoire d’échange compressée dans la RAM. Elle économise de la place au prix d’un peu de temps processeur."],
  "explain.pressure": ["PSI is the share of the last 10 seconds during which tasks were delayed by a lack of this resource.","La pression (PSI) mesure la part des 10 dernières secondes pendant laquelle des tâches ont attendu cette ressource."],
  "explain.power": ["CPU package power is measured at the processor package. Per-app energy is estimated from each app's CPU share, not measured directly.","La puissance du processeur est mesurée au niveau du boîtier. L’énergie par application est estimée selon sa part de processeur, sans mesure directe."],
  "explain.api": ["An estimate of what the same AI usage would cost at API rates. It is not an invoice or your subscription charge.","Estimation du coût de la même utilisation aux tarifs de l’API. Ce n’est ni une facture ni le montant de votre abonnement."],
  "explain.coverage": ["The share of the period with valid observations. Missing observations are not counted as healthy.","Part de la période couverte par des mesures valides. Les périodes sans mesure ne sont pas considérées comme saines."],
  "ui.what.this.means.a4bcc": ["What this means","Comprendre"],
  "ui.fleet.summary.52086": ["Fleet summary","Vue du parc"],
  "ui.private.host.fd00f": ["Private host","Hôte privé"],
  "ui.reporting.c9e57": ["Reporting","En ligne"],
  "ui.measured.95503": ["Measured","Mesuré le"],
  "ui.fleetglass.card.preview.8484c": ["Fleetglass card preview","Aperçu de la carte Fleetglass"],
  "live.seconds_ago": ["{seconds} s ago", "il y a {seconds} s"],
  "change.apt": ["APT", "APT"],
  "change.service": ["Service", "Service"],
  "change.boot": ["Boot", "Démarrage"],
  "change.git": ["Git", "Git"],
  "change.container": ["Container", "Conteneur"],
  "change.marker": ["Marker", "Marqueur"],
  "change.heat": ["Heat", "Chaleur"],
  "palette.view": ["View", "Vue"],
  "palette.host": ["Host", "Hôte"],
  "palette.agent": ["Agent", "Agent"],
  "choice.all": ["All", "Tous"],
  "overview.by_project_range": ["by project, {range}", "par projet, {range}"],
  "overview.received": ["{amount} received", "{amount} reçus"],
  "agents.filtered_count": ["{shown} of {total}", "{shown} sur {total}"],
  "overview.busy_now": ["Busy, now", "Occupé maintenant"],
  "overview.cpu_package": ["CPU package", "Puissance CPU"],
  "overview.package_temp": ["Package temp", "Température du processeur"],
  "overview.ai_cost": ["AI cost", "Coût de l’IA"],
  "overview.package_not_wall": ["Package, not wall", "Processeur seul, sans la consommation de la machine entière"],
  "overview.of_total": ["{used} of {total}", "{used} sur {total}"],
  "agent.unknown": ["Unknown agent", "Agent inconnu"],
  "agent.source.otlp": ["OpenTelemetry", "OpenTelemetry"],
  "agent.source.log": ["session log", "journal de session"],
  "agent.source.process": ["process", "processus"],
  "agent.source.worktree": ["worktree", "arbre de travail"],
  "chart.trend": ["{label} trend over selected period", "Tendance de {label} sur la période choisie"],
  "chart.checks_normal": ["{good} of {total} checks normal", "{good} contrôles normaux sur {total}"],
  "chart.no_checks": ["No checks reported", "Aucun contrôle signalé"],
  "chart.response": ["{label} response time over {period} in milliseconds, with {failed} failed or unknown samples", "Temps de réponse de {label} sur {period} en millisecondes, avec {failed} mesures en échec ou inconnues"],

  "network.sign_in_required": ["Sign in required", "Connexion requise"],
  "network.request_failed": ["Request failed ({status})", "Échec de la requête ({status})"],
  "network.waiting_sample": ["Waiting for the first host sample.", "En attente de la première mesure de l’hôte."],
  "network.unavailable": ["Connection unavailable", "Connexion indisponible"],
  "auth.invalid_token": ["The read token was not accepted.", "Le jeton de lecture a été refusé."],
  "system.pending_updates": ["Pending updates", "Mises à jour en attente"],
  "system.reboot_required": ["Reboot required", "Redémarrage requis"],
  "system.ssh_logins": ["SSH logins", "Connexions SSH"],
  "system.sudo_use": ["sudo use", "Usage de sudo"],
  "system.failed_logins": ["Failed logins", "Connexions échouées"],
  "system.all_interface_ports": ["All-interface ports", "Ports sur toutes les interfaces"],
  "system.sensitive_changes": ["Sensitive file changes", "Fichiers sensibles modifiés"],
  "system.updates_summary": ["{total} total · {security} security", "{total} au total · {security} de sécurité"],
  "system.yes": ["Yes", "Oui"],
  "system.no": ["No", "Non"],
  "system.in_24h": ["{count} in 24 h", "{count} sur 24 h"],
  "system.ssh_last": ["{count} in 24 h · last {user} ({source})", "{count} sur 24 h · dernier : {user} ({source})"],
  "system.none": ["none", "aucun"],
  "system.none_reported": ["None reported", "Aucun signalé"],
  "system.since_previous": ["{count} since previous sample", "{count} depuis la mesure précédente"],
  "system.sudo_user": ["sudo user", "utilisateur sudo"],
  "system.failed_source": ["failed login source", "origine des connexions échouées"],
  "system.backup_latest": ["{count} files · latest {age} ago", "{count} fichiers · dernier il y a {age}"],
  "system.repo_commit": ["commit {age} ago", "commit il y a {age}"],
  "system.oom_victim": ["OOM victim", "Processus arrêté par manque de mémoire"],
  "system.unknown_unit": ["Unknown unit", "Service inconnu"],
  "system.unknown_process": ["Unknown process", "Processus inconnu"],
  "system.no_message": ["No message", "Aucun message"],
  "system.down": ["Down", "En panne"],

  "state.operational": ["Operational", "Opérationnel"],
  "state.degraded": ["Degraded", "Dégradé"],
  "state.down": ["Down", "En panne"],
  "state.unknown": ["Unknown", "Inconnu"],
  "coverage.none": ["No measurements in {period}", "Aucune mesure sur {period}"],
  "coverage.measured": ["Measured {duration} of {period}", "{duration} de mesures sur {period}"],
  "coverage.less_than_minute": ["less than 1 min", "moins de 1 min"],
  "coverage.day": ["day", "jour"],
  "public.down.one": ["{count} service down", "{count} service en panne"],
  "public.down.many": ["{count} services down", "{count} services en panne"],
  "public.degraded.one": ["{count} service degraded", "{count} service dégradé"],
  "public.degraded.many": ["{count} services degraded", "{count} services dégradés"],
  "public.unknown.one": ["{count} service unknown", "{count} service sans état connu"],
  "public.unknown.many": ["{count} services unknown", "{count} services sans état connu"],
  "public.all_operational": ["All systems operational", "Tous les services fonctionnent"],
  "public.unavailable": ["Status is temporarily unavailable.", "L’état des services est temporairement indisponible."],
  "fleet.summary": ["{hosts} hosts · {live} live · {silent} silent · {offline} offline", "{hosts} hôtes · {live} en direct · {silent} silencieux · {offline} hors ligne"],

  "machine.memory_used": ["Memory used", "Mémoire utilisée"],
  "machine.memory_total": ["Memory total", "Mémoire totale"],
  "machine.swap_used": ["Swap used", "Mémoire d’échange utilisée"],
  "machine.swap_total": ["Swap total", "Mémoire d’échange totale"],
  "machine.busy": ["Busy", "Occupé"],
  "machine.user": ["User", "Utilisateur"],
  "machine.system": ["System", "Système"],
  "machine.iowait": ["I/O wait", "Attente d’E/S"],
  "machine.steal": ["Steal", "Temps volé"],
  "machine.load": ["Load 1 / 5 / 15", "Charge sur 1 / 5 / 15 min"],
  "machine.used": ["Used", "Utilisée"],
  "machine.page_cache": ["Page cache", "Cache des pages"],
  "machine.buffers": ["Buffers", "Tampons"],
  "machine.shared": ["Shared", "Partagée"],
  "machine.swap": ["Swap", "Mémoire d’échange"],
  "machine.zram": ["zram memory", "Mémoire zram"],
  "machine.pressure": ["Pressure, 10 s", "Pression, 10 s"],
  "machine.now": ["{section} now", "{section} maintenant"],
  "machine.core": ["Core {number}", "Cœur {number}"],
  "machine.in_out": ["{inbound} in · {outbound} out", "{inbound} en réception · {outbound} en émission"],
  "machine.read_write": ["{read} read · {write} write", "{read} en lecture · {write} en écriture"],
  "machine.written_today": ["{size} written today", "{size} écrits aujourd’hui"],
  "machine.disk_space": ["{used} used · {free} free", "{used} utilisés · {free} libres"],
  "machine.since_boot": ["{amount} since boot", "{amount} depuis le démarrage"],
  "machine.traffic_today": ["Today {inbound} in · {outbound} out", "Aujourd’hui : {inbound} en réception · {outbound} en émission"],
  "machine.restarts": ["{count} restarts", "{count} redémarrages"],
  "machine.cpu_notes": ["Host CPU is {cpu} across {cores} cores. App CPU is expressed as a percentage of one core and can exceed 100%. Measured package power is {power} W.", "Le processeur de l’hôte est utilisé à {cpu} sur {cores} cœurs. L’usage d’une application est rapporté à un cœur et peut dépasser 100 %. La puissance mesurée du processeur est de {power} W."],
  "machine.gpu_notes": ["GPU utilization depends on driver support. Per-app GPU use is not measured by this driver.", "La mesure du GPU dépend du pilote. Ce pilote ne mesure pas l’usage du GPU par application."],
  "machine.history": ["{section} history", "Historique : {section}"],
  "machine.wear": ["{percent} wear", "Usure : {percent}"],
  "machine.media_errors": ["{count} media errors", "{count} erreurs du support"],
  "machine.inodes": ["{used} / {total} inodes", "{used} / {total} inodes"],
  "machine.drive_write": ["drive write bytes/s", "écriture disque en octets/s"],
  "machine.receive": ["receive bytes/s", "réception en octets/s"],
  "machine.used_bytes": ["used bytes", "octets utilisés"],
  "machine.host_cpu": ["host CPU %", "processeur de l’hôte %"],
  "machine.gpu_percent": ["GPU %", "GPU %"],

  "app.other": ["Other / not attributed", "Autres / non attribué"],
  "app.show_12h": ["Show 12 h", "Afficher 12 h"],
  "app.show_48h": ["Show 48 h", "Afficher 48 h"],
  "app.details": ["{name} details", "Détails de {name}"],
  "project.idle.one": ["{count} idle server", "{count} serveur inactif"],
  "project.idle.many": ["{count} idle servers", "{count} serveurs inactifs"],
  "project.process.one": ["{count} process", "{count} processus"],
  "project.process.many": ["{count} processes", "{count} processus"],
  "project.copy_command_for": ["Copy stop command for {name}, PID {pid}", "Copier la commande d’arrêt de {name}, PID {pid}"],
  "project.none": ["none", "aucun"],
  "storage.partial": ["Partial / unreadable", "Partiel / illisible"],
  "storage.waiting_growth": ["Waiting for 7 d", "En attente de 7 j de mesures"],
  "choice.storage": ["Storage", "Stockage"],
  "choice.gpu": ["GPU", "GPU"],
  "choice.unit": ["Unit", "Service"],
  "choice.container": ["Container", "Conteneur"],
  "choice.other": ["Other", "Autres"],
  "choice.all_types": ["All types", "Tous les types"],
  "choice.energy": ["Energy", "Énergie"],
  "ui.unknown.bc781": ["Unknown","Inconnu"],
  "ui.no.sample.f797c": ["No sample","Aucune mesure"],
  "ui.certificate.735ad": ["certificate","certificat"],
  "ui.not.measured.6e95b": ["Not measured","Non mesuré"],
  "ui.waiting.33d30": ["Waiting","En attente"],
  "ui.all.interfaces.8d058": ["All interfaces","Toutes les interfaces"],
  "ui.api.equivalent.3504a": ["API equivalent","Équivalent API"],
  "ui.reported.share.c6c3e": ["Reported share","Part attribuée"],
  "ui.connection.lost.62583": ["Connection lost","Connexion perdue"],
  "ui.stale.data.2e635": ["Stale data","Données anciennes"],
  "ui.connecting.c1f3b": ["Connecting","Connexion"],
  "ui.live.65c82": ["Live","En direct"],
  "ui.ports.not.measured.d4001": ["Ports not measured","Ports non mesurés"],
  "ui.copied.8e3df": ["Copied","Copiée"],
  "ui.copy.stop.command.e9e3a": ["Copy stop command","Copier la commande d’arrêt"],
  "ui.preferences.saved.in.this.browser.8c56a": ["Preferences saved in this browser.","Préférences enregistrées dans ce navigateur."],
  "ui.display.574ff": ["Display","Affichage"],
  "ui.language.89b86": ["Language","Langue"],
  "ui.temperature.0a906": ["Temperature","Température"],
  "ui.network.rate.e2691": ["Network rate","Débit réseau"],
  "ui.cpu.usage.d52b4": ["CPU usage","Usage du processeur"],
  "ui.navigation.cf03c": ["Navigation","Navigation"],
  "ui.choose.visible.tabs.settings.always.remains.avai.6bae1": ["Choose visible tabs. Settings always remains available.","Choisissez les onglets visibles. Réglages reste toujours accessible."],
  "ui.sections.7ff5a": ["Sections","Sections"],
  "ui.hide.cards.you.do.not.need.6eab6": ["Hide cards you do not need.","Masquez les cartes dont vous n’avez pas besoin."],
  "ui.reset.44c57": ["Reset","Réinitialiser"],
  "ui.restore.server.defaults.and.show.all.tabs.and.se.be9b1": ["Restore server defaults and show all tabs and sections.","Rétablir les valeurs par défaut du serveur et réafficher tous les onglets et toutes les sections."],
  "ui.reset.settings.9ba8d": ["Reset settings","Rétablir les réglages"],
  "ui.export.a.1200.630.pixel.summary.8c4bb": ["Export a 1200 × 630 pixel summary.","Exportez un aperçu de 1200 × 630 pixels."],
  "ui.card.4d4ce": ["Card","Carte"],
  "ui.content.4f9be": ["Content","Contenu"],
  "ui.hide.host.and.project.names.9f067": ["Hide host and project names","Masquer les noms des hôtes et des projets"],
  "ui.the.card.is.drawn.in.this.browser.no.project.nam.00d8c": ["The card is drawn in this browser. No project name is included.","La carte est dessinée dans ce navigateur. Aucun nom de projet n’y figure."],
  "ui.download.png.0c830": ["Download PNG","Télécharger le PNG"],
  "ui.preview.f1fbb": ["Preview","Aperçu"],
  "agent.state.working": ["Working", "En cours"],
  "agent.state.running": ["Running", "En cours d’exécution"],
  "agent.state.waiting_for_input": ["Waiting for input", "En attente de saisie"],
  "agent.state.rate_limited": ["Rate limited", "Limite de débit atteinte"],
  "agent.state.erroring": ["Erroring", "En erreur"],
  "agent.state.ended": ["Ended", "Terminée"],
  "agent.state.unknown": ["Unknown", "Inconnu"],
  "agent.state.stale_worktree": ["Stale worktree", "Arbre de travail inactif"],
  "agent.reason.process_alive": ["Process alive; session activity unavailable", "Processus actif ; activité de la session indisponible"],
  "agent.reason.inactive_ended": ["No process matched; no activity for over 30 minutes", "Aucun processus associé ; aucune activité depuis plus de 30 minutes"],
  "agent.reason.quota_reached": ["Quota reached", "Quota atteint"],
  "agent.reason.awaiting_input": ["Process alive, no recent activity", "Processus actif, sans activité récente"],
  "agent.reason.turn_ended": ["Last turn ended; no process matched", "Dernier échange terminé ; aucun processus associé"],
  "agent.reason.recent_activity": ["Recent activity", "Activité récente"],
  "agent.reason.inactive_unknown": ["No recent activity", "Aucune activité récente"],
  "agent.reason.stale_worktree": ["No agent process and no directory change for 3 days", "Aucun processus d’agent ni changement du répertoire depuis 3 jours"],
  "quota.limit_reached": ["Limit reached", "Limite atteinte"],
  "quota.ahead": ["Ahead of pace", "Au-dessus du rythme prévu"],
  "quota.within": ["Within pace", "Dans le rythme prévu"],
  "quota.five_hour": ["5 hours", "5 heures"],
  "quota.seven_day": ["7 days", "7 jours"],
  "quota.minutes": ["{minutes} min window", "Fenêtre de {minutes} min"],
  "quota.hours": ["{hours} h", "{hours} heures"],
  "quota.days": ["{days} d", "{days} j"],
  "quota.not_reported": ["{plan} · not reported", "{plan} · non signalé"],
  "quota.claude_hint": ["set claude_budget_5h / claude_budget_week to estimate", "Définir claude_budget_5h / claude_budget_week pour estimer"],
  "quota.reported": ["reported by {agent}", "signalé par {agent}"],
  "quota.estimated": ["estimated", "estimé"],
  "observation.root": ["Root filesystem", "Système de fichiers racine"],
  "observation.cpu_package": ["CPU package power", "Puissance du processeur"],
  "observation.cpu": ["CPU", "Processeur"],
  "observation.memory": ["Memory", "Mémoire"],
  "observation.sensor": ["Sensor {name}", "Capteur {name}"],
  "observation.service": ["Service {name}", "Service {name}"],
  "observation.container": ["Container {name}", "Conteneur {name}"],
  "observation.network": ["Network {name}", "Réseau {name}"],
  "observation.certificate": ["Certificate {name}", "Certificat {name}"],
  "observation.oom": ["Out-of-memory kill", "Arrêt par manque de mémoire"],
  "observation.closed_target": ["Closed target", "Cible fermée"],
  "observation.offsite_copy": ["Offsite copy", "Copie hors site"],
  "observation.check": ["Unnamed check", "Sonde sans nom"],
  "value.unknown_model": ["Unknown model", "Modèle inconnu"],
  "value.no_reason": ["No reason reported", "Aucun motif signalé"],

  "ui.project.worktree.cbe27": ["Project / worktree","Projet / arbre de travail"],
  "ui.state.reason.b5f5d": ["State / reason","État / motif"],
  "ui.cpu.by.process.f62e5": ["CPU by process","Processeur par processus"],
  "ui.memory.by.process.d76ce": ["Memory by process","Mémoire par processus"],
  "ui.method.88306": ["Method","Méthode"],
  "ui.coverage.80e13": ["Coverage","Couverture"],
  "ui.journal.last.24.h.acc7b": ["journal, last 24 h", "journal, dernières 24 h"],
  "ui.local.host.24.h.41ae3": ["local host · 24 h", "hôte local · 24 h"],
  "ui.configured.sets.latest.file.6a4e5": ["configured sets · latest file", "ensembles configurés · dernier fichier"],
  "ui.configured.checks.79704": ["configured checks", "contrôles configurés"],
  "ui.local.git.refs.no.fetch.e53ec": ["local Git refs · no fetch", "références Git locales · sans fetch"],
  "ui.security.backups.and.local.service.checks.from.t.46d5c": ["Security, backups and local service checks from this host.", "Sécurité, sauvegardes et contrôles des services locaux de cet hôte."],
  "ui.by.process.3b481": ["by process", "par processus"],
  "ui.pace.steady.use.until.reset.0fd4b": ["pace = steady use until reset", "rythme = usage régulier jusqu’à la remise à zéro"],
  "ui.period.170a2": ["Period", "Période"],
  "ui.cpu.percent.and.package.power.56857": ["CPU percent and package power", "Usage du processeur et puissance mesurée"],
  "ui.selected.period.7cc59": ["selected period", "période choisie"],
  "ui.systemd.tls.dfd8b": ["systemd · TLS", "systemd · TLS"],
  "ui.per.interface.now.39f4b": ["per interface, now", "par interface, maintenant"],
  "ui.docker.9f114": ["Docker", "Docker"],
  "ui.every.reported.session.including.unknown.and.ina.203c5": ["Every reported session, including unknown and inactive states.", "Toutes les sessions signalées, y compris celles dont l’état est inconnu ou inactif."],
  "ui.hardware.and.resource.measurements.from.the.sele.51456": ["Hardware and resource measurements from the selected host.", "Mesures matérielles et ressources de l’hôte sélectionné."],
  "ui.subsystem.b1a08": ["Subsystem", "Sous-système"],
  "ui.cpu.package.w.0d73c": ["CPU % · package W", "Processeur % · puissance W"],
  "ui.bytes.daf52": ["bytes", "octets"],
  "ui.rates.now.totals.since.boot.a9730": ["rates now · totals since boot", "débits actuels · totaux depuis le démarrage"],
  "ui.host.measurements.082dd": ["host measurements", "Mesures de l’hôte"],
  "ui.measured.counter.changes.b99ac": ["measured counter changes", "Variations des compteurs mesurés"],
  "ui.per.container.289a4": ["per container", "par conteneur"],
  "ui.coverage.6dafb": ["coverage", "couverture"],
  "ui.measured.now.7782e": ["measured now", "mesuré maintenant"],
  "ui.every.running.process.attributed.to.a.unit.conta.6b1d2": ["Every running process attributed to a unit, container or agent session.", "Chaque processus actif attribué à un service, à un conteneur ou à une session d’agent."],
  "ui.rank.apps.by.bd839": ["Rank apps by", "Classer les applications par"],
  "ui.system.units.containers.ports.and.certificates.388a0": ["System units, containers, ports and certificates.", "Services système, conteneurs, ports et certificats."],
  "ui.configured.units.616b0": ["configured units", "services configurés"],
  "ui.per.socket.807ef": ["per socket", "par socket"],
  "ui.by.bound.address.9bdff": ["by bound address", "par adresse d’écoute"],
  "ui.change.period.aaa89": ["Change period", "Période des changements"],
  "ui.ai.usage.and.measured.cpu.package.energy.84fbb": ["AI usage and measured CPU package energy.", "Usage de l’IA et énergie mesurée du processeur."],
  "ui.cpu.package.only.56d4b": ["CPU package only", "Boîtier du processeur uniquement"],
  "ui.all.token.types.9ba49": ["all token types", "tous types de jetons"],
  "ui.http.and.internal.probe.availability.with.unknow.d319a": ["HTTP and internal probe availability, with unknown periods kept visible.", "Disponibilité des sondes HTTP et internes, avec les périodes sans mesure visibles."],


  "value.unknown": ["Unknown", "Inconnu"],
  "status.check_down.one": ["{count} check down", "{count} contrôle en échec"],
  "status.check_down.many": ["{count} checks down", "{count} contrôles en échec"],
  "status.critical.one": ["{count} critical issue", "{count} problème critique"],
  "status.critical.many": ["{count} critical issues", "{count} problèmes critiques"],
  "status.watch.one": ["{count} thing to watch", "{count} élément à surveiller"],
  "status.watch.many": ["{count} things to watch", "{count} éléments à surveiller"],
  "status.stale.one": ["{count} stale check", "{count} contrôle ancien"],
  "status.stale.many": ["{count} stale checks", "{count} contrôles anciens"],
  "status.unknown.one": ["{count} check unknown", "{count} contrôle sans état connu"],
  "status.unknown.many": ["{count} checks unknown", "{count} contrôles sans état connu"],
  "status.running_well": ["Running well", "Tout fonctionne"],
  "status.unknown": ["Status unknown", "État inconnu"],
  "status.down": ["DOWN", "EN PANNE"],
  "status.critical": ["CRITICAL", "CRITIQUE"],
  "status.watch": ["WATCH", "À SURVEILLER"],
  "status.stale": ["STALE", "ANCIEN"],
  "status.unknown_tag": ["UNKNOWN", "INCONNU"],
  "status.last_observed": ["Last observed {at} · {detail}", "Dernière mesure {at} · {detail}"],
  "status.summary": ["{reported} · {unknown} · {stale} · {agents}", "{reported} · {unknown} · {stale} · {agents}"],
  "status.reported.one": ["{count} reported check", "{count} contrôle signalé"],
  "status.reported.many": ["{count} reported checks", "{count} contrôles signalés"],
  "status.unknown_count.one": ["{count} unknown", "{count} inconnu"],
  "status.unknown_count.many": ["{count} unknown", "{count} inconnus"],
  "status.stale_count.one": ["{count} stale", "{count} ancien"],
  "status.stale_count.many": ["{count} stale", "{count} anciens"],
  "status.agent_count.one": ["{count} AI agent", "{count} agent IA"],
  "status.agent_count.many": ["{count} AI agents", "{count} agents IA"],
  "insight.cpu": ["CPU above 80% of one core for 10 minutes", "Processeur au-dessus de 80 % d’un cœur pendant 10 minutes"],
  "insight.disk": ["Disk writes above 10 MiB/s for 15 minutes", "Écritures disque au-dessus de 10 Mio/s pendant 15 minutes"],
  "insight.network": ["Network traffic above 10 MiB/s for 15 minutes", "Trafic réseau au-dessus de 10 Mio/s pendant 15 minutes"],
  "insight.memory": ["Memory grew steadily by at least 100 MiB in one hour", "Mémoire en hausse régulière d’au moins 100 Mio sur une heure"],
  "insight.idle_dev_server": ["Development server idle for at least 3 hours", "Serveur de développement inactif depuis au moins 3 heures"],
  "insight.runaway_dev_server": ["Development server above 80% CPU without a new connection for 30 minutes", "Serveur de développement au-dessus de 80 % du processeur sans nouvelle connexion depuis 30 minutes"],
  "ui.no.observations.reported.e045a": [
    "No observations reported.",
    "Aucune mesure signalée."
  ],
  "ui.status.bae7d": [
    "Status",
    "État"
  ],
  "ui.ai.agents.2b51b": [
    "AI agents",
    "Agents IA"
  ],
  "ui.nothing.needs.attention.b7acc": [
    "Nothing needs attention",
    "Rien à signaler"
  ],
  "ui.failed.units.cc1be": [
    "Failed units",
    "Services en échec"
  ],
  "ui.containers.e040a": [
    "Containers",
    "Conteneurs"
  ],
  "ui.overview": ["Overview", "Vue d’ensemble"],
  "ui.power": ["Power", "Puissance"],
  "ui.temperature": ["Temperature", "Température"],
  "ui.agents.64acf": [
    "Agents",
    "Agents"
  ],
  "ui.to.watch.f8978": [
    "To watch",
    "À surveiller"
  ],
  "ui.no.agent.sessions.reported.f9f1f": [
    "No agent sessions reported.",
    "Aucune session d’agent signalée."
  ],
  "ui.project.f6f4d": [
    "Project",
    "Projet"
  ],
  "ui.cpu.ff221": [
    "CPU",
    "Processeur"
  ],
  "ui.memory.89c8a": [
    "Memory",
    "Mémoire"
  ],
  "ui.tokens.c38c6": [
    "Tokens",
    "Jetons"
  ],
  "ui.sessions.come.from.opentelemetry.and.local.sessi.9a18f": [
    "Sessions come from OpenTelemetry and local session logs. Process figures require attribution.",
    "Les sessions proviennent d’OpenTelemetry et des journaux locaux. Les mesures des processus dépendent de leur attribution."
  ],
  "ui.no.quota.observations.reported.42bea": [
    "No quota observations reported.",
    "Aucune mesure de quota signalée."
  ],
  "ui.tokens.33914": [
    "tokens",
    "jetons"
  ],
  "ui.no.token.history.for.this.period.0e2da": [
    "No token history for this period.",
    "Aucun historique de jetons pour cette période."
  ],
  "ui.recent.errors.b0f52": [
    "Recent errors",
    "Erreurs récentes"
  ],
  "system.error.one": ["{count} error", "{count} erreur"],
  "system.error.many": ["{count} errors", "{count} erreurs"],
  "system.oom.one": ["{count} OOM kill", "{count} arrêt par manque de mémoire"],
  "system.oom.many": ["{count} OOM kills", "{count} arrêts par manque de mémoire"],
  "system.container_restart.one": ["{count} container restart", "{count} redémarrage de conteneur"],
  "system.container_restart.many": ["{count} container restarts", "{count} redémarrages de conteneurs"],
  "ui.priority.err.and.above.full.text.search.comes.la.7980f": [
    "Priority err and above. Full text search comes later.",
    "Priorité « erreur » et supérieure. La recherche en texte intégral viendra plus tard."
  ],
  "ui.security.f25ce": [
    "Security",
    "Sécurité"
  ],
  "ui.backups.530cc": [
    "Backups",
    "Sauvegardes"
  ],
  "ui.status.files.and.internal.probes.46db9": [
    "Status files and internal probes",
    "Fichiers d’état et sondes internes"
  ],
  "ui.repositories.c91e6": [
    "Repositories",
    "Dépôts"
  ],
  "ui.system.bc079": [
    "System",
    "Système"
  ],
  "ui.ai.agents.running.now.4d737": [
    "AI agents running now",
    "Agents IA en cours"
  ],
  "ui.view.all.931e1": [
    "View all",
    "Tout voir"
  ],
  "ui.quotas.34ed5": [
    "Quotas",
    "Quotas"
  ],
  "ui.cpu.and.package.power.688e3": [
    "CPU and package power",
    "Processeur et puissance du boîtier"
  ],
  "ui.cpu.a2ccb": [
    "CPU %",
    "Processeur %"
  ],
  "ui.package.w.right.axis.78a38": [
    "Package W (right axis)",
    "Puissance du boîtier (axe droit)"
  ],
  "ui.cpu.package.only.5747c": [
    "CPU package only ·",
    "Boîtier du processeur uniquement ·"
  ],
  "ui.kwh.measured.today.1658a": [
    "kWh measured today",
    "kWh mesurés aujourd’hui"
  ],
  "ui.what.changed.7e4b9": [
    "What changed",
    "Changements récents"
  ],
  "ui.view.all.changes.cee9a": [
    "View all changes",
    "Voir tous les changements"
  ],
  "ui.services.and.certificates.e34a8": [
    "Services and certificates",
    "Services et certificats"
  ],
  "ui.network.53ebc": [
    "Network",
    "Réseau"
  ],
  "ui.sessions.e11e3": [
    "Sessions",
    "Sessions"
  ],
  "ui.agent.5ce2e": [
    "Agent",
    "Agent"
  ],
  "ui.state.a7250": [
    "State",
    "État"
  ],
  "ui.machine.65bb1": [
    "Machine",
    "Machine"
  ],
  "ui.heat.attribution.1afc5": [
    "Heat attribution",
    "Origine de la chaleur"
  ],
  "ui.at.threshold.crossing.top.cpu.apps.5da3e": [
    "At threshold crossing · top CPU apps",
    "Au franchissement du seuil · applications les plus actives"
  ],
  "ui.busy.now.20bdd": [
    "busy now",
    "occupé maintenant"
  ],
  "ui.memory.and.swap.3591a": [
    "Memory and swap",
    "Mémoire et échange"
  ],
  "ui.disks.and.inodes.d72dc": [
    "Disks and inodes",
    "Disques et inodes"
  ],
  "ui.temperatures.and.fans.014e2": [
    "Temperatures and fans",
    "Températures et ventilateurs"
  ],
  "ui.smart.disk.health.f1b19": [
    "SMART disk health",
    "État SMART des disques"
  ],
  "ui.network.interfaces.ae625": [
    "Network interfaces",
    "Interfaces réseau"
  ],
  "ui.no.measurements.reported.079d3": [
    "No measurements reported.",
    "Aucune mesure signalée."
  ],
  "ui.history.90ccd": [
    "History",
    "Historique"
  ],
  "ui.volumes.39c0e": [
    "Volumes",
    "Volumes"
  ],
  "ui.memory.pressure.ff290": [
    "Memory pressure",
    "Pression mémoire"
  ],
  "ui.psi.reports.the.share.of.time.tasks.waited.for.m.0bf4c": [
    "PSI reports the share of time tasks waited for memory. Page cache can be reclaimed when applications need it.",
    "La pression mémoire indique la part du temps où des tâches ont attendu de la mémoire. Le cache des pages peut être libéré lorsque les applications en ont besoin."
  ],
  "ui.traffic.by.interface.c13ca": [
    "Traffic by interface",
    "Trafic par interface"
  ],
  "ui.container.resources.9b1e1": [
    "Container resources",
    "Ressources des conteneurs"
  ],
  "ui.block.i.o.884a2": [
    "Block I/O",
    "E/S disque"
  ],
  "ui.restarts.81727": [
    "Restarts",
    "Redémarrages"
  ],
  "ui.measurement.notes.bdd57": [
    "Measurement notes",
    "Notes de mesure"
  ],
  "ui.apps.behind.this.figure.612b6": [
    "Apps behind this figure",
    "Applications derrière ce chiffre"
  ],
  "ui.per.app.gpu.use.is.not.measured.by.this.driver.f2aef": [
    "Per-app GPU use is not measured by this driver.",
    "Ce pilote ne mesure pas l’usage du GPU par application."
  ],
  "ui.apps.d9a36": [
    "Apps",
    "Applications"
  ],
  "ui.app.groups.measured.8135f": [
    "app groups measured",
    "groupes d’applications mesurés"
  ],
  "ui.cpu.is.a.percentage.of.one.core.current.shares.a.69782": [
    "CPU is a percentage of one core. Current shares are reconciled to host totals; the remainder is Other / not attributed. Energy is an estimate from CPU time and measured package power. Container network is shown when its namespace can be read.",
    "L’usage du processeur est rapporté à un cœur. Les parts sont rapprochées du total de l’hôte ; le reste apparaît dans « Autres / non attribué ». L’énergie est estimée à partir du temps processeur et de la puissance mesurée. Le trafic d’un conteneur apparaît lorsque son espace réseau est accessible."
  ],
  "ui.resource.owners.a20f8": [
    "Resource owners",
    "Responsables des ressources"
  ],
  "ui.type.3deb7": [
    "Type",
    "Type"
  ],
  "app.matching.one": ["{count} matching app", "{count} application correspondante"],
  "app.matching.many": ["{count} matching apps", "{count} applications correspondantes"],
  "ui.ranked.apps.887df": [
    "Ranked apps",
    "Applications classées"
  ],
  "ui.app.fc4a6": [
    "App",
    "Application"
  ],
  "ui.disk.read.write.23868": [
    "Disk read / write",
    "Lecture / écriture disque"
  ],
  "ui.network.in.out.e4044": [
    "Network in / out",
    "Réseau entrant / sortant"
  ],
  "ui.energy.estimate.48ed6": [
    "Energy estimate",
    "Énergie estimée"
  ],
  "ui.share.09ca5": [
    "Share",
    "Partager"
  ],
  "ui.no.apps.match.this.search.or.type.584a6": [
    "No apps match this search or type.",
    "Aucune application ne correspond à cette recherche ou à ce type."
  ],
  "ui.close.bbfa7": [
    "Close",
    "Fermer"
  ],
  "ui.processes.ed730": [
    "Processes",
    "Processus"
  ],
  "ui.disk.today.6bb6e": [
    "Disk today:",
    "Disque aujourd’hui :"
  ],
  "ui.top.apps.hour.by.hour.367fc": [
    "Top apps, hour by hour",
    "Applications en tête, heure par heure"
  ],
  "ui.last.48.hours.11ccc": [
    "Last 48 hours",
    "Dernières 48 heures"
  ],
  "ui.disk.ef995": [
    "Disk",
    "Disque"
  ],
  "ui.hourly.leaders.appear.after.an.hour.of.measureme.092ce": [
    "Hourly leaders appear after an hour of measurements.",
    "Les applications en tête apparaîtront après une heure de mesures."
  ],
  "ui.services.5cbd5": [
    "Services",
    "Services"
  ],
  "ui.systemd.units.dc52c": [
    "Systemd units",
    "Services systemd"
  ],
  "ui.docker.containers.ba4cd": [
    "Docker containers",
    "Conteneurs Docker"
  ],
  "ui.listening.ports.e2117": [
    "Listening ports",
    "Ports en écoute"
  ],
  "ui.no.listening.ports.reported.bcaa5": [
    "No listening ports reported.",
    "Aucun port en écoute signalé."
  ],
  "ui.tls.certificates.c50be": [
    "TLS certificates",
    "Certificats TLS"
  ],
  "ui.changes.8aa57": [
    "Changes",
    "Changements"
  ],
  "ui.package.service.boot.merge.container.and.configu.fe439": [
    "Package, service, boot, merge, container and configured marker events.",
    "Événements liés aux paquets, services, démarrages, fusions Git, conteneurs et marqueurs configurés."
  ],
  "ui.change.journal.642e1": [
    "Change journal",
    "Journal des changements"
  ],
  "ui.no.changes.in.this.period.88120": [
    "No changes in this period.",
    "Aucun changement sur cette période."
  ],
  "ui.costs.799b3": [
    "Costs",
    "Coûts"
  ],
  "ui.energy.437bc": [
    "Energy",
    "Énergie"
  ],
  "ui.today.24345": [
    "Today",
    "Aujourd’hui"
  ],
  "ui.kwh.72c28": [
    "kWh",
    "kWh"
  ],
  "ui.this.month.1b478": [
    "This month",
    "Ce mois-ci"
  ],
  "ui.today.cost.193fc": [
    "Today cost",
    "Coût du jour"
  ],
  "ui.month.cost.dedfe": [
    "Month cost",
    "Coût du mois"
  ],
  "ui.at.08550": [
    "At",
    "À"
  ],
  "ui.kwh.observed.intervals.only.cpu.package.only.not.148fe": [
    "/kWh · observed intervals only · CPU package only, not wall power.",
    "/kWh · périodes mesurées uniquement · processeur seul, sans la consommation de la machine entière."
  ],
  "ui.ai.cost.by.project.26d43": [
    "AI cost by project",
    "Coût de l’IA par projet"
  ],
  "ui.no.priced.usage.in.this.period.e9408": [
    "No priced usage in this period.",
    "Aucun usage chiffré sur cette période."
  ],
  "ui.api.equivalent.is.an.estimate.not.a.bill.subscri.101cc": [
    "API equivalent is an estimate, not a bill. Subscription allocation follows token share.",
    "L’équivalent API est une estimation, pas une facture. Les abonnements sont répartis selon les jetons utilisés."
  ],
  "ui.tokens.by.model.92019": [
    "Tokens by model",
    "Jetons par modèle"
  ],
  "ui.no.model.usage.in.this.period.b130a": [
    "No model usage in this period.",
    "Aucun usage de modèle sur cette période."
  ],
  "ui.fleet.e1991": [
    "Fleet",
    "Parc"
  ],
  "ui.hosts.53103": [
    "Hosts",
    "Hôtes"
  ],
  "ui.reporting.now.4fd91": [
    "reporting now",
    "avec des mesures récentes"
  ],
  "ui.ai.usage.24.h.fae99": [
    "AI usage · 24 h",
    "Usage de l’IA · 24 h"
  ],
  "ui.tokens.across.all.hosts.f1666": [
    "tokens across all hosts",
    "jetons sur tous les hôtes"
  ],
  "ui.ai.cost.24.h.f1814": [
    "AI cost · 24 h",
    "Coût de l’IA · 24 h"
  ],
  "ui.api.equivalent.estimate.6f668": [
    "API equivalent estimate",
    "Estimation au tarif de l’API"
  ],
  "ui.label.74341": [
    "Label",
    "Étiquette"
  ],
  "ui.sort.adc4e": [
    "Sort",
    "Tri"
  ],
  "ui.no.hosts.match.these.filters.ab0a5": [
    "No hosts match these filters.",
    "Aucun hôte ne correspond à ces filtres."
  ],
  "ui.ai.usage.by.host.24.h.09bbd": [
    "AI usage by host · 24 h",
    "Usage de l’IA par hôte · 24 h"
  ],
  "ui.ai.usage.by.project.24.h.eb6fa": [
    "AI usage by project · 24 h",
    "Usage de l’IA par projet · 24 h"
  ],
  "ui.command.palette.7b6b5": [
    "Command palette",
    "Palette de commandes"
  ],
  "ui.search.views.hosts.or.agents.86df3": [
    "Search views, hosts or agents",
    "Rechercher des vues, des hôtes ou des agents"
  ],
  "ui.search.commands.5ae3a": [
    "Search commands",
    "Rechercher des commandes"
  ],
  "ui.daily.availability.for.30.days.5072a": [
    "Daily availability for 30 days",
    "Disponibilité quotidienne sur 30 jours"
  ],
  "ui.availability.681b5": [
    "Availability",
    "Disponibilité"
  ],
  "ui.view.all.probes.9f6c8": [
    "View all probes",
    "Voir toutes les sondes"
  ],
  "ui.uptime.6aafa": [
    "Uptime",
    "Disponibilité"
  ],
  "ui.no.probes.configured.yet.e62a6": [
    "No probes configured yet.",
    "Aucune sonde configurée."
  ],
  "ui.24.hours.cb478": [
    "24 hours",
    "24 heures"
  ],
  "ui.7.days.0a8dd": [
    "7 days",
    "7 jours"
  ],
  "ui.30.days.7d427": [
    "30 days",
    "30 jours"
  ],
  "ui.http.f40b2": [
    "HTTP",
    "HTTP"
  ],
  "ui.latest.decd7": [
    "Latest",
    "Dernière mesure"
  ],
  "ui.ms.26cc3": [
    "ms",
    "ms"
  ],
  "ui.dns.3ddc2": [
    "DNS",
    "DNS"
  ],
  "ui.connect.b6546": [
    "Connect",
    "Connexion"
  ],
  "ui.tls.d91e1": [
    "TLS",
    "TLS"
  ],
  "ui.first.byte.87d97": [
    "First byte",
    "Premier octet"
  ],
  "ui.certificate.10de4": [
    "Certificate",
    "Certificat"
  ],
  "ui.days.5548a": [
    "days",
    "jours"
  ],
  "ui.response.time.e3f4e": [
    "Response time",
    "Temps de réponse"
  ],
  "ui.response.6e617": [
    "Response",
    "Réponse"
  ],
  "ui.p50.42e31": [
    "p50",
    "p50"
  ],
  "ui.p95.e375f": [
    "p95",
    "p95"
  ],
  "ui.failure.16566": [
    "Failure",
    "Échec"
  ],
  "ui.fleetglass.2e022": [
    "Fleetglass",
    "Fleetglass"
  ],
  "ui.service.status.7efad": [
    "Service status",
    "État des services"
  ],
  "ui.availability.over.the.past.30.days.8e7a8": [
    "Availability over the past 30 days.",
    "Disponibilité sur les 30 derniers jours."
  ],
  "ui.no.public.services.configured.854c9": [
    "No public services configured.",
    "Aucun service public configuré."
  ],
  "ui.30.day.uptime.84b41": [
    "30 day uptime",
    "Disponibilité sur 30 jours"
  ],
  "ui.current.state.8e20a": [
    "Current state",
    "État actuel"
  ],
  "ui.host.3960e": [
    "Host",
    "Hôte"
  ],
  "ui.views.24be6": [
    "Views",
    "Vues"
  ],
  "ui.settings.c7f73": [
    "Settings",
    "Réglages"
  ],
  "ui.search.or.jump.to.commands.control.or.command.k.26860": [
    "Search or jump to commands (Control or Command K)",
    "Rechercher ou accéder à une commande (Ctrl ou Commande K)"
  ],
  "ui.search.or.jump.to.commands.ctrl.k.50228": [
    "Search or jump to commands · Ctrl/⌘ K",
    "Rechercher ou accéder à une commande · Ctrl/⌘ K"
  ],
  "ui.k.64d86": [
    "⌘K",
    "⌘K"
  ],
  "ui.sign.out.dc164": [
    "Sign out",
    "Se déconnecter"
  ],
  "ui.connecting.fd3e7": [
    "Connecting…",
    "Connexion…"
  ],
  "ui.sign.in.to.fleetglass.c7302": [
    "Sign in to Fleetglass",
    "Connexion à Fleetglass"
  ],
  "ui.enter.the.server.read.token.to.view.the.cockpit.9cbd9": [
    "Enter the server read token to view the cockpit.",
    "Saisissez le jeton de lecture du serveur pour voir le tableau de bord."
  ],
  "ui.read.token.7180b": [
    "Read token",
    "Jeton de lecture"
  ],
  "ui.sign.in.ada2e": [
    "Sign in",
    "Se connecter"
  ],
  "ui.retry.9f5cd": [
    "Retry",
    "Réessayer"
  ],
  "ui.no.host.samples.yet.f109c": [
    "No host samples yet",
    "Aucune mesure d’hôte pour le moment"
  ],
  "ui.start.the.agent.to.populate.this.cockpit.8a383": [
    "Start the agent to populate this cockpit.",
    "Démarrez l’agent pour alimenter ce tableau de bord."
  ],
  "ui.view.fleet.8d8a0": [
    "View fleet →",
    "Voir le parc →"
  ],
  "ui.what.stands.out.691fa": [
    "What stands out",
    "À retenir"
  ],
  "ui.fixed.rules.no.ai.f3669": [
    "Fixed rules · no AI",
    "Règles fixes · sans IA"
  ],
  "ui.no.rule.has.crossed.its.duration.threshold.43f0c": [
    "No rule has crossed its duration threshold.",
    "Aucun seuil de durée dépassé."
  ],
  "ui.cpu.gt.80.for.10.min.memory.100.mib.for.1.h.with.145d4": [
    "CPU &gt; 80% for 10 min; memory +100 MiB for 1 h without a 5% fall; disk write or measured network &gt; 10 MiB/s for 15 min; dev server idle for 3 h or busy without new connections for 30 min. Insights are observations, not alerts.",
    "Processeur au-dessus de 80 % pendant 10 min ; mémoire en hausse de 100 Mio sur 1 h sans baisse de 5 % ; écritures disque ou trafic réseau mesuré au-dessus de 10 Mio/s pendant 15 min ; serveur de développement inactif pendant 3 h ou actif sans nouvelle connexion pendant 30 min. Ces observations ne sont pas des alertes."
  ],
  "ui.projects.53e89": [
    "Projects",
    "Projets"
  ],
  "ui.agents.and.long.running.processes.under.configur.14995": [
    "Agents and long-running processes under configured project and worktree roots.",
    "Agents et processus persistants dans les projets et arbres de travail configurés."
  ],
  "ui.idle.dev.servers.ad057": [
    "Idle dev servers",
    "Serveurs de développement inactifs"
  ],
  "ui.15.min.without.cpu.above.1.or.a.new.connection.f9521": [
    "15 min without CPU above 1% or a new connection",
    "15 min sans usage du processeur supérieur à 1 % ni nouvelle connexion"
  ],
  "ui.latest.process.inventory.is.stale.5912a": [
    "Latest process inventory is stale.",
    "Le dernier inventaire des processus est ancien."
  ],
  "ui.stopping.these.would.free.e4751": [
    "Stopping these would free",
    "Leur arrêt libérerait"
  ],
  "ui.of.process.memory.and.ports.38762": [
    "of process memory and ports",
    "de mémoire de processus et les ports"
  ],
  "ui.estimate.from.current.process.rss.copy.a.command.58c52": [
    "Estimate from current process RSS. Copy a command per process; Fleetglass never runs it.",
    "Estimation d’après la mémoire actuelle des processus. Vous pouvez copier une commande par processus ; Fleetglass ne l’exécute jamais."
  ],
  "ui.pid.08323": [
    "· PID",
    "· PID"
  ],
  "ui.storage.map.ec0f7": [
    "Storage map",
    "Carte du stockage"
  ],
  "ui.largest.directories.under.configured.roots.to.tw.53937": [
    "Largest directories under configured roots, to two visible levels.",
    "Les plus grands répertoires des racines configurées, sur deux niveaux."
  ],
  "ui.hourly.scan.b64e7": [
    "Hourly scan ·",
    "Analyse horaire ·"
  ],
  "ui.directory.4b892": [
    "Directory",
    "Répertoire"
  ],
  "ui.size.b7152": [
    "Size",
    "Taille"
  ],
  "ui.7.d.growth.36d43": [
    "7 d growth",
    "Croissance sur 7 j"
  ],
  "ui.directory.totals.include.descendants.symbolic.li.373ca": [
    "Directory totals include descendants. Symbolic links are not followed; inaccessible paths and timed-out scans are marked partial. No file names are collected.",
    "Les tailles comprennent les sous-répertoires. Les liens symboliques ne sont pas suivis ; les chemins inaccessibles et les analyses interrompues sont signalés comme partiels. Aucun nom de fichier n’est collecté."
  ],
  "ui.waiting.for.storage.scan.699ef": [
    "Waiting for storage scan",
    "En attente de l’analyse du stockage"
  ],
  "ui.the.agent.scans.configured.roots.once.per.hour.2fffc": [
    "The agent scans configured roots once per hour.",
    "L’agent analyse les racines configurées une fois par heure."
  ],
  "ui.theme.a797e": [
    "Theme",
    "Thème"
  ],
  "ui.light.a36ef": [
    "Light",
    "Clair"
  ],
  "ui.dark.ae1ef": [
    "Dark",
    "Sombre"
  ]
} as const;
export type MessageKey = keyof typeof messages;
export const navigationLabels = {
  overview: "ui.overview", fleet: "ui.fleet.e1991", agents: "ui.agents.64acf", apps: "ui.apps.d9a36",
  projects: "ui.projects.53e89", machine: "ui.machine.65bb1", services: "ui.services.5cbd5",
  system: "ui.system.bc079", changes: "ui.changes.8aa57", costs: "ui.costs.799b3", uptime: "ui.uptime.6aafa",
} as const satisfies Record<string, MessageKey>;
export const sectionLabels = {
  CPU: "ui.cpu.ff221", Memory: "ui.memory.89c8a", Disk: "ui.disk.ef995", Network: "ui.network.53ebc",
  Power: "ui.power", Temperature: "ui.temperature", Agents: "ui.agents.64acf", Services: "ui.services.5cbd5",
  Costs: "ui.costs.799b3", Backups: "ui.backups.530cc", Security: "ui.security.f25ce",
} as const satisfies Record<string, MessageKey>;

const choiceMessages: Record<string, MessageKey> = {
  Light: "ui.light.a36ef", Dark: "ui.dark.ae1ef", Overview: "ui.overview", CPU: "ui.cpu.ff221",
  Memory: "ui.memory.89c8a", Disk: "ui.disk.ef995", Network: "ui.network.53ebc",
  GPU: "choice.gpu", Containers: "ui.containers.e040a", Storage: "choice.storage",
  Energy: "choice.energy", Unit: "choice.unit", Container: "choice.container",
  Agent: "ui.agent.5ce2e", Other: "choice.other", "All types": "choice.all_types", All: "choice.all",
};
export function choiceLabel(code: string): string {
  return choiceMessages[code] ? t(choiceMessages[code]) : code;
}
let currentLanguage: Language | null = null;
export function setLanguage(language: Language) { currentLanguage = language; }
export function activeLanguage(): Language {
  if (currentLanguage) return currentLanguage;
  try { return JSON.parse(localStorage.getItem("fleetglass.preferences.v1") || "{}").language === "fr" ? "fr" : "en"; }
  catch { return "en"; }
}
export function t(key: MessageKey, paramsOrLanguage?: Record<string, string | number> | Language, requestedLanguage?: Language): string {
  const language = typeof paramsOrLanguage === "string" ? paramsOrLanguage : requestedLanguage ?? activeLanguage();
  const params = typeof paramsOrLanguage === "string" ? undefined : paramsOrLanguage;
  const template = messages[key][language === "fr" ? 1 : 0];
  if (!params) return template;
  return template.replace(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g, (_, name: string) => {
    const value = params[name];
    return typeof value === "number" ? new Intl.NumberFormat(language === "fr" ? "fr-FR" : "en-GB").format(value) : String(value ?? `{${name}}`);
  });
}
export function plural(count: number, singular: MessageKey, multiple: MessageKey, params: Record<string, string | number> = {}): string {
  return t(count === 1 ? singular : multiple, {count, ...params});
}
