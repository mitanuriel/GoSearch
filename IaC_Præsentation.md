# Infrastructure as Code (IaC) i GoSearch-projektet
## DevOps Eksamen - Mundtlig Præsentation

---

## Indledning

God dag! I dag vil jeg præsentere Infrastructure as Code implementationen i mit GoSearch-projekt. GoSearch demonstrerer en moderne tilgang til Infrastructure as Code, hvor hele infrastrukturen er defineret som kode gennem forskellige konfigurationsfiler. Dette gør det muligt for os at versionere, teste og reproducere hele vores infrastruktur konsistent på tværs af forskellige miljøer, hvilket er en fundamental DevOps-praksis.

Jeg vil nu gennemgå de forskellige komponenter i projektet, hvor jeg åbner hver fil og forklarer dens rolle i vores IaC-strategi.

---

## 1. Container Orchestration med Docker Compose

Lad mig starte med at åbne vores `docker-compose.yml` fil, som er hjørnestenen i hele vores Infrastructure as Code strategi.

### docker-compose.yml - Produktionsmiljøet

Denne fil definerer hele vores produktionsmiljøs arkitektur som kode. Når jeg ser på filen, kan I se at den indeholder definitionen af fem centrale services, der tilsammen udgør vores komplette applikationsstack.

For det første har vi vores app-service, som er selve Go-applikationen. Her kan I se at vi har sat en memory limit på 256 megabytes, hvilket er vigtigt for at forhindre at en enkelt container bruger alle serverens ressourcer. Containeren er konfigureret med automatisk genstart ved fejl gennem restart-policyen "always", hvilket betyder at hvis applikationen crasher, vil Docker automatisk starte den igen uden manuel intervention.

Den næste service er PostgreSQL, vores relationelle database. Som I kan se i konfigurationen, bruger vi persistent storage gennem et named volume kaldet postgres_data. Dette er kritisk vigtigt, fordi det sikrer at vores data overlever container restarts og opdateringer. Jeg har også optimeret memory settings med en limit på 200 megabytes og specificeret shared buffers og work memory for at tune databasens performance.

Den tredje service er Elasticsearch, som håndterer vores full-text search funktionalitet. Her har jeg været nødt til at tune den ganske grundigt, fordi Elasticsearch normalt er meget ressourcekrævende. I kan se at jeg har sat Java heap size til kun 128 megabytes både for minimum og maximum, og jeg har deaktiveret X-Pack security da vi kører i et lukket netværk. Den totale memory limit er sat til 512 megabytes.

De sidste to services er Prometheus og Grafana, som udgør vores monitoring-stack. Prometheus scraper metrics fra vores applikation hvert femtende sekund, mens Grafana visualiserer disse metrics gennem dashboards. Det interessante her er at både Prometheus-konfigurationen og Grafana-dashboards er defineret som kode gennem volume mounts, hvilket jeg vil vende tilbage til senere.

Fra et DevOps-perspektiv er denne fil ekstremt værdifuld. For det første er hele vores stack defineret deklarativt, hvilket betyder at vi beskriver ønsket tilstand frem for at skrive scripts der udfører handlinger. Vi bruger depends_on til at sikre korrekt opstartrækkefølge, så applikationen ikke starter før databasen er klar. Miljøvariabler injiceres fra en git-ignored `.env` fil, hvilket sikrer at sensitive data ikke committes til vores repository. Og vigtigst af alt - med denne ene fil kan vi recreate hele vores produktionsmiljø på få minutter.

### docker-compose.dev.yml - Udviklingsmiljøet

Lad mig nu åbne `docker-compose.dev.yml`, som er vores development setup. Denne fil er betydeligt simplere end production versionen, hvilket er bevidst. Den giver udviklere mulighed for hurtigt at spinde et miljø op lokalt til test og udvikling. Alle udviklere får nøjagtig det samme miljø, hvilket eliminerer "det virker på min maskine" problemet. Dog kan jeg se at filen er meget kort, hvilket indikerer at det muligvis stadig er work in progress eller at vi primært bruger production compose-filen lokalt.

---

## 2. Container Definition med Dockerfile

Lad mig nu åbne vores `Dockerfile`, som definerer hvordan vores container image bygges. Dette er et fantastisk eksempel på Infrastructure as Code, fordi selve byggeprocessen er defineret deklarativt.

### Multi-stage Build Strategi

Som I kan se, bruger vi en multi-stage build strategi, hvilket er en best practice i Docker-verdenen. Lad mig forklare hvorfor dette er så vigtigt.

I den første stage, som jeg kalder "builder", starter vi med et Golang 1.24.0 Alpine image. Dette image indeholder alle de build tools vi behøver for at kompilere vores Go-applikation. Jeg installerer build-base og postgresql-dev pakker, fordi vores applikation bruger PostgreSQL driver der kræver C-bindings, og derfor skal vi have CGO aktiveret. I denne stage kopierer vi vores go.mod og go.sum filer først, kører go mod download, og derefter kopierer vi vores source code og kompilerer applikationen. Denne rækkefølge er bevidst, fordi Docker cacher hvert lag, så hvis vi kun ændrer source code, behøver vi ikke at downloade dependencies igen.

Den anden stage er vores runtime image, som starter fra et rent Alpine 3.21.3 image. Dette er hvor magien sker. Alpine Linux er ekstremt lille, kun omkring fem megabytes i base størrelse, hvilket gør vores finale image meget mere kompakt. Fra builder-stagen kopierer vi kun den kompilerede binær og de nødvendige runtime filer. Vi installerer Node.js og npm, fordi vi kører database migrations med Knex, og vi installerer PostgreSQL client til at kunne kommunikere med databasen.

Et meget vigtigt sikkerhedsaspekt her er at jeg opretter og skifter til en non-root user kaldet "nonroot". Dette er kritisk for sikkerhed, fordi hvis en angriber på en eller anden måde får adgang til containeren, har de ikke root-rettigheder og kan derfor ikke gøre så meget skade. Dette er en security best practice der ofte overses.

Fra et DevOps-perspektiv giver denne multi-stage approach os flere fordele. For det første reducerer vi vores finale image size drastisk, fordi vi ikke inkluderer alle build tools i runtime imaget. Dette betyder hurtigere deployments og mindre båndbredde-forbrug. For det andet har vi pinnet versions på både Golang og Alpine, hvilket sikrer reproducibility - vi får altid nøjagtig samme miljø når vi bygger. For det tredje optimerer layer caching vores CI/CD pipeline, fordi uændrede lag ikke behøver at rebuildes.

En lille note til forbedring er at nogle Alpine pakker ikke er version-pinned, hvilket Hadolint advarer om. I en enterprise setting ville jeg fixe dette, men for et mindre projekt er risikoen acceptabel.

Det interessante er også at vi inkluderer Knex migrations direkte i imaget, hvilket betyder at database schema er bundlet med applikationskoden. Dette sikrer at de altid er synkroniseret.

---

## 3. Startup Orchestration med entrypoint.sh

### Automated Bootstrap Process

**Funktionalitet:**
1. Kører database schema migrations via Knex
2. Migrerer data fra SQLite til PostgreSQL
3. Starter Go-applikationen

**DevOps-perspektiv:**
- ✅ **Database as Code:** Schema changes versioneres og deployeres automatisk
- ✅ **Zero-downtime migrations:** Migrations køres før app start
- ✅ **Environment flexibility:** Bruger environment variables for konfiguration
- ✅ **Idempotency:** Migrations kan køres gentagne gange sikkert
- ✅ **Error handling:** `set -e` stopper ved fejl

**Vigtighed:** Dette sikrer at database altid er synkroniseret med applikationskode

---

## 4. Database Migrations - Knex

### knexfile.js
**Konfigurerer database connections for forskellige miljøer:**
- Development (localhost)
- Docker (container networking)

### Migration Files
**Eksempel: 20250417134210_create_users_and_pages.js**
- Opretter tabeller deklarativt
- Inkluderer default admin user
- Reversible (`up` og `down` funktioner)

**DevOps-perspektiv:**
- ✅ **Schema versioning:** Hver ændring er timestamped og versioneret
- ✅ **Rollback capability:** Down migrations giver rollback mulighed
- ✅ **Reproducibility:** Samme schema på tværs af alle miljøer
- ✅ **Git integration:** Migrations i version control
- ✅ **Audit trail:** Historik af alle schema ændringer

Nu vil jeg åbne `entrypoint.sh`, som er vores automated bootstrap script. Dette lille script er faktisk ekstremt vigtigt for hele vores deployment strategi.

### Automated Bootstrap Process

Lad mig gennemgå hvad der sker når en container starter. Først sætter vi "set -e", hvilket betyder at scriptet stopper ved første fejl. Dette er god praksis, fordi vi ikke vil have at containeren starter hvis migrations fejler.

Scriptet kører gennem tre faser. For det første navigerer det til vores knex-migrations mappe og sætter alle nødvendige environment variables for database connection. Derefter kører det "npx knex migrate:latest", som udfører alle pending database migrations. Dette er fantastisk, fordi det betyder at hver gang vi deployer en ny version af applikationen, bliver database schema automatisk opdateret til at matche den nye kode.

Den anden fase kører et custom migration script, "migrate-sqlite-to-postgres.js", som flytter data fra vores gamle SQLite database til PostgreSQL. Dette var nødvendigt fordi vi migrerede fra SQLite til Postgres på et tidspunkt i projektets historie, og vi ville sikre at eksisterende data blev bevaret.

Til sidst startes selve Go-applikationen med exec kommandoen. Exec er vigtig her, fordi det erstatter shell-processen med Go-processen, hvilket betyder at vores Go-app får PID 1 og kan modtage signals korrekt fra Docker.

Fra et DevOps-perspektiv er dette utrolig værdifuldt. Vi har Database as Code - alle schema ændringer er versionerede og deployes automatisk sammen med applikationen. Migrations kan køres gentagne gange sikkert, fordi Knex holder styr på hvilke migrations der allerede er kørt. Dette giver os zero-downtime migrations, fordi database schema opdateres før den nye applikations-version starter. Og vigtigst af alt - dette sikrer at database altid er synkroniseret med applikationskode, så vi aldrig får version-mismatch problemer.

---

## 4. Database Migrations med Knex

Lad mig nu åbne `knexfile.js`, som konfigurerer vores database connections. Dette er endnu et eksempel på Infrastructure as Code, hvor database konfiguration er defineret i en versioneret fil.

### Knex Konfiguration

Som I kan se i knexfile, har vi to separate konfigurationer - en for development miljøet hvor databasen kører på localhost, og en for Docker miljøet hvor databasen er tilgængelig via service navnet "postgres". Dette er et godt eksempel på environment-specific konfiguration der stadig er defineret som kode. Begge environments bruger environment variables for credentials, hvilket betyder at vi kan ændre passwords uden at ændre kode.

### Migration Filer

Lad mig åbne en af vores migration filer for at vise hvordan Database as Code fungerer i praksis. Hvis jeg åbner "20250417134210_create_users_and_pages.js", kan I se at vi har to funktioner - en "up" funktion der opretter tabeller, og en "down" funktion der fjerner dem igen.

I up-funktionen opretter vi først en users tabel med felter for id, username, email og password. Derefter indsætter vi en default admin bruger med credentials fra environment variables. Til sidst opretter vi en pages tabel til at holde web-scraped content. Hver migration er timestamped i filnavnet, hvilket giver os en kronologisk historik af alle database ændringer.

I vores projekt har jeg seks migrations, og hver af dem repræsenterer en specifik ændring i vores schema. Vi har "create_processed_searches" til search logging, "create_users_and_pages" til user management, "add_search_indexes" som en performance optimization, "add_password_changed_column" som tilføjer en security feature, "force_password_reset" der enforcer en security policy, og "change_pages_primary_key" som refaktorerer vores schema.

Fra et DevOps-perspektiv giver denne tilgang os enormt mange fordele. Hver schema ændring er versioneret med et timestamp og tracked i Git, hvilket giver os fuld audit trail af alle database ændringer. Vi har rollback capability gennem down migrations, selvom jeg vil indrømme at vi i praksis sjældent bruger dem. Vi har garanteret reproducibility - samme schema på tværs af development, staging og production. Og måske vigtigst af alt - når vi laver en pull request der ændrer database schema, kan mine teammedlemmer review ændringen før den merges, præcis som med kode.

---

## 5. Monitoring as Code

Monitoring er ofte noget der tilføjes som en efterfølger, men i dette projekt har jeg integreret det fra start som en del af vores Infrastructure as Code. Lad mig vise hvordan.

### Prometheus Konfiguration

Hvis jeg åbner `prometheus/prometheus.yml`, kan I se vores Prometheus konfiguration. Dette er en meget simpel fil, men den demonstrerer et vigtigt princip - monitoring targets er defineret som kode, ikke konfigureret gennem et UI.

Som I kan se, har vi en scrape config for vores gosearch job, hvor vi speciferer metrics path som "/metrics" og target som "go-app:8080". Bemærk at vi bruger Docker service navnet her, ikke en IP-adresse, hvilket betyder at service discovery håndteres automatisk af Docker networking. Vi scraper metrics hvert femtende sekund, hvilket giver os god granularitet uden at overbelaste systemet.

Dette betyder at hvis vi tilføjer en ny service der skal monitores, tilføjer vi bare en ny scrape config i denne fil, committer den til Git, og deployer. Ingen manuel konfiguration i et UI. Denne konfiguration er versioneret, reviewable og reproducerbar.

### Grafana Provisioning

Grafana provisioning er hvor det bliver virkelig interessant. Traditionelt ville man logge ind i Grafana UI, manuelt oprette datasources og oprette dashboards ved hånd. Men vi gør det anderledes.

Hvis jeg åbner `grafana/provisioning/datasources/prometheus.yml`, kan I se at vi auto-konfigurerer to Prometheus datasources - en lokal til development og en production datasource der peger på vores gosearch1.dk server. Dette betyder at når Grafana starter første gang, er disse datasources allerede konfigureret. Ingen manuel setup nødvendig.

Og hvis jeg åbner `grafana/provisioning/dashboards/dashboards.yml`, kan I se at vi auto-loader dashboards fra JSON filer i en specifik mappe. Dette betyder at vi kan versionere vores dashboards som JSON filer i Git. Hvis jeg laver ændringer i et dashboard, eksporterer jeg det som JSON, committer det, og alle andre får den samme dashboard næste gang de deployer.

Fra et DevOps-perspektiv er dette fantastisk. For det første er monitoring ikke en efterfølger men integreret fra start. For det andet kan dashboards peer-reviewes gennem pull requests - hvis jeg laver et nyt dashboard, kan mine teammedlemmer se ændringerne og kommentere før de merges. For det tredje har vi rollback capability - hvis et dashboard opdatering ødelægger noget, kan vi bare git revert. Og endelig sikrer det consistency - alle miljøer får samme monitoring setup.

Dette er essensen af GitOps - hele vores monitoring stack er deklareret i Git og deployes automatisk.

---

## 6. CI/CD Pipelines med GitHub Actions

Nu vil jeg vise hvordan vi har automatiseret hele vores build og deployment proces gennem GitHub Actions. Dette er måske det mest imponerende aspekt af vores Infrastructure as Code implementation.

### Continuous Deployment Pipeline

Lad mig åbne `.github/workflows/continuous_deployment.yml`. Dette er vores komplette deployment pipeline til production, og den demonstrerer hvordan vi har Infrastructure as Code hele vejen gennem til deployment.

Pipeline består af to jobs - build og deploy. Lad mig gennemgå hvad der sker i build-fasen. Når vi pusher til main branch, checker workflow'en vores kode ud fra Git, sætter Docker Buildx op for at kunne lave multi-platform builds, og logger ind i GitHub Container Registry. Derefter bygger den vores Docker image ved hjælp af vores Dockerfile, og pusher den til GHCR tagget som "latest". Alt dette sker automatisk uden nogen manuel intervention.

Det interessante her er at hele build processen er defineret deklarativt. Vi bruger vores Dockerfile som build specification, og GitHub Actions orchestrerer execution. Images bliver automatisk versioneret og stored i et registry.

Nu kommer deploy-fasen, som kun kører hvis build var successful, og kun på main branch. Først tilføjer vi SSH key til runneren, så vi kan connecte til vores production server. Vi tilføjer serveren til known_hosts for at undgå man-in-the-middle attacks. Derefter transfererer vi vores docker-compose.dev.yml fil til serveren via SCP.

Næste skridt er interessant - workflow'en checker om Docker er installeret på serveren, og hvis ikke, installerer den det automatisk. Dette er Infrastructure as Code taget til det næste niveau - selve deployment target provisioning er automatiseret. Det samme gør den for docker-compose.

Derefter stopper og fjerner vi eksisterende containere, navigerer til GoSearch mappen på serveren, puller den nye image fra registry, og kører "docker-compose up -d --force-recreate", hvilket starter alle services med den nye version. Til sidst cleaner vi op ved at pruune ubrugte containers og images for at spare disk space.

Fra et DevOps-perspektiv er denne pipeline ekstremt værdifuld. Vi har fuld automated deployment - en push til main branch resulterer automatisk i et production deployment inden for få minutter. Vi har infrastructure provisioning som en del af pipeline - Docker installeres automatisk hvis det mangler. Vi har built-in cleanup automation for at forhindre disk space issues. Og security er håndteret gennem GitHub secrets for SSH keys.

Dog skal jeg være ærlig om at der er plads til forbedring. Vi har ikke implementeret blue-green eller canary deployment strategies, hvilket ville give os zero-downtime deployments. Og vi har ikke automatisk rollback ved fejl - hvis noget går galt i production, skal vi manuelt gå tilbage til en tidligere version.

### Continuous Integration Pipeline

Lad mig nu åbne `.github/workflows/actions.yml`, som er vores CI pipeline. Denne workflow kører på alle pushes og pull requests til både main og develop branches, hvilket giver os early feedback på code quality.

Pipeline har tre jobs - build, lint og sonarcloud. I build-jobbet ser I noget interessant - vi starter en Elasticsearch service container automatisk. Dette er fantastisk, fordi det betyder at vores integration tests kan køre mod en rigtig Elasticsearch instance, ikke en mock. Vi sætter environment variables op for Elasticsearch connection, kører alle vores unit tests, kører integration tests separat, og til sidst bygger vi applikationen for at sikre at den kompilerer.

Vi kører også Hadolint, som er en linter for Dockerfiles. Dette fanger common mistakes og security issues i vores container definition.

I lint-jobbet validerer vi først at Go version i GitHub Actions matcher versionen i vores go.mod fil. Dette er vigtigt for consistency. Derefter kører vi golangci-lint, som checker for code quality issues, security problems og style violations.

I sonarcloud-jobbet kører vi static code analysis for at fange security vulnerabilities, code smells og maintainability issues.

Fra et DevOps-perspektiv giver denne pipeline os automated testing på hver code change, quality gates der forhindrer dårlig kode i at blive merged, service containers der giver os realistic test miljøer, og version consistency checks. Det fantastiske er at hvis en pull request fejler disse tests, kan den ikke merges, hvilket enforcer kvalitet.

### Automated Dependency Management

Lad mig åbne `dependabot.yml`, som konfigurerer automated dependency updates. Her har vi sat Dependabot op til at monitore to ecosystems - GitHub Actions og Go modules.

Hver mandag tjekker Dependabot for nye versioner af vores dependencies og opretter automatisk pull requests med opdateringerne. Vi har limiteret det til max tre åbne PRs ad gangen for ikke at blive overwhelmed. Og vi targeter develop branch, så opdateringerne går gennem vores normale review og test process før de kommer i production.

Dette er Infrastructure as Code applied til dependency management. Fra et DevOps-perspektiv reducerer dette vores security debt ved at holde dependencies up-to-date, minimerer breaking changes ved at opdatere ofte i små steps frem for sjældent i store spring, og automatiserer en kedelig manual task. Det er god DevOps praksis at automatisere det du kan automatisere.

---

## 7. Configuration Management og Best Practices

Lad mig nu vise nogle af de mindre filer der stadig spiller vigtige roller i vores IaC strategi.

### Environment Variables Strategy

I vores docker-compose.yml bruger vi environment variables konsekvent. Som I kan se, injicerer vi CONN_STR for database connection, APP_ENV for at specifiere miljø, SESSION_SECRET for session encryption, og OPENWEATHER_API_KEY for vores vejr API integration. Alle disse kommer fra en .env fil der ikke er commitet til Git.

Dette følger twelve-factor app methodology, hvor konfiguration holdes separat fra kode. Det giver os flexibility til at bruge samme Docker image i forskellige miljøer med forskellige configurations. Security er forbedret fordi secrets ikke er hardcoded. Og vi kan ændre konfiguration uden at rebuilde images.

### .dockerignore

Hvis jeg åbner `.dockerignore`, kan I se at vi excluder database filer, SQL filer og backup mappen fra vores build context. Dette er vigtigt af flere grunde. For det første optimerer det build hastighed ved at reducere context size - Docker behøver ikke at uploade disse filer til build daemon. For det andet forbedrer det security ved at forhindre at sensitive data accidentally inkluderes i images. Og for det tredje reducerer det final image size.

Dette er en lille fil, men den demonstrerer attention to detail i vores IaC strategi.

---

## 8. Kritisk Analyse - Hvad Mangler?

Nu vil jeg være kritisk overfor mit eget projekt og diskutere hvad der mangler for at dette kan kaldes enterprise-grade Infrastructure as Code.

### Cloud Infrastructure Provisioning

Det første og mest åbenlyse er at vi ikke har Terraform eller Pulumi til at provisionere cloud infrastructure. I øjeblikket deployer vi til en manuelt opsat server. I en enterprise setting ville vi have Terraform kode der provisionerer VMs, netværk, load balancers, og managed services automatisk. Server provisioning er delvist automatiseret i vores GitHub Actions workflow, men det er ikke ideelt.

Hvis jeg skulle implementere dette, ville jeg lave Terraform modules til at provisionere hele vores infrastruktur på for eksempel AWS eller Azure. Så ville server configuration, networking rules, security groups - alt sammen være defineret som kode.

### Kubernetes

For det andet bruger vi Docker Compose, som ikke er en production-grade orchestrator. Docker Compose er fantastisk til development og mindre deployments, men det mangler critical features som horizontal scaling, advanced health checks, automatic failover og self-healing. I en større skala ville vi bruge Kubernetes med Helm charts til at definere vores deployments.

Kubernetes ville give os mulighed for at skalere services op og ned automatisk baseret på load, distribuere vores applikation på tværs af multiple nodes for high availability, og give os rolling updates med zero downtime. Men det kommer med significant complexity, så for et projekt af denne størrelse er Docker Compose faktisk et fornuftigt valg.

### Secrets Management

For det tredje er vores secrets management basic. Vi bruger .env filer og GitHub secrets, hvilket fungerer, men det er ikke enterprise-grade. I production ville jeg bruge HashiCorp Vault, AWS Secrets Manager eller Kubernetes Sealed Secrets til at håndtere secrets på en mere sikker måde med encryption at rest, rotation policies og audit logging.

### Netværk og Backup

Vi har ingen eksplicitte network policies defineret, hvilket betyder at alle vores containers kan tale med hinanden. I en security-conscious setup ville vi have network segmentation defineret som kode. Vi har heller ingen automated backup strategi defineret som kode - backups køres manuelt, hvilket ikke er ideelt.

Og vi mangler en disaster recovery plan som kode. Hvis vores server går ned, skulle vi manuelt recreate infrastrukturen. Med Terraform ville vi kunne spin up et helt nyt miljø på minutter.

---

## 9. Styrker ved Projektets IaC Implementation

Men lad mig også fremhæve hvad vi gør rigtig godt, fordi der er faktisk meget at være stolt af her.

### Reproducibility

En af de største styrker er reproducibility. Med vores setup kan jeg recreate hele miljøet fra scratch på minutter. Jeg cloner repositoryet, kører "docker-compose up", og hele stacken - applikation, database, Elasticsearch, monitoring - alt sammen kommer op med korrekt konfiguration. Database schema deployes automatisk gennem migrations. Monitoring kommer op med pre-configured dashboards. Dette er ekstremt værdifuldt både for development og disaster recovery.

### Version Control og Collaboration

Alt vores infrastruktur er i Git, hvilket giver os complete change history. Jeg kan se hvem der ændrede hvad og hvornår. Hvis noget går galt, kan jeg rollback til en tidligere version. Og måske vigtigst af alt - infrastruktur ændringer går gennem samme review process som kode. Når jeg laver en pull request der ændrer docker-compose konfiguration, kan mine teammedlemmer reviewe og kommentere før den merges.

### Automation

Vi har høj grad af automation gennem vores CI/CD pipelines. Tests kører automatisk, quality checks enforces standards, dependencies opdateres automatisk, og deployments til production sker automatisk når vi merger til main. Dette reducerer manuel toil og eliminerer human error i deployment processen.

### Self-Documentation

En stor fordel ved Infrastructure as Code er at koden er dokumentationen. Hvis nogen vil vide hvordan vores monitoring er sat op, kan de læse prometheus.yml. Hvis de vil vide hvilke services vi kører, kan de læse docker-compose.yml. Dette er meget bedre end dokumentation i et Word dokument der bliver outdated efter to uger.

### Testability

Vores infrastructure er testbar. Vores CI pipeline spinner up ephemeral Elasticsearch containers til at teste integration. Vi kan teste infrastructure changes i feature branches før de merges. Dette ville være meget svært med manuelt configured infrastructure.

### Security Posture

Fra et security perspektiv gør vi flere ting rigtigt. Vi kører non-root containers, hvilket er best practice. Vi har dependency scanning gennem Dependabot, så vi bliver alertet om vulnerabilities. Vi har static code analysis gennem SonarCloud. Og vi har secrets management, selvom det kunne være bedre.

---

## 10. DevOps Principper Demonstreret

Lad mig nu reflektere over hvordan dette projekt demonstrerer fundamentale DevOps principper.

### Infrastructure as Code

Det mest åbenlyse er Infrastructure as Code princippet selv. Alle vores komponenter er defineret deklarativt gennem konfigurationsfiler. Docker Compose definerer vores services, Dockerfile definerer vores container images, Knex migrations definerer vores database schema, Prometheus og Grafana konfiguration definerer vores monitoring. Alt sammen er versioneret, reviewable og reproducerbar. Dette er kernen i moderne DevOps praksis.

### Continuous Integration og Continuous Deployment

Vi har fuldt automatiseret CI/CD. Hver code change trigrer automated tests og quality checks. Når vi merger til main, deployer systemet automatisk til production. Dette giver os rapid feedback loops og reducer time-to-market for nye features. I stedet for at skulle vente på en release cycle, kan vi deploye flere gange om dagen hvis nødvendigt.

### Monitoring and Observability

Vi har integreret monitoring fra dag ét. Prometheus scraper metrics fra vores applikation, og Grafana visualiserer dem. Dette giver os observability into system health og performance. Og fordi monitoring er defineret som kode, er det ikke en efterfølger men en integral del af vores infrastruktur.

### Security as Code

Selvom der er plads til forbedring, demonstrerer projektet security as code principper. Container security gennem non-root users, dependency scanning gennem Dependabot, static analysis gennem SonarCloud, og secrets management gennem environment variables. Security er ikke noget vi tænker på til sidst, men noget der er baked in fra start.

### Collaboration og Knowledge Sharing

Infrastructure as Code faciliterer collaboration. Når alt er i Git, kan vi bruge pull requests, code reviews og pair programming på infrastruktur præcis som på applikations kode. Dette bryder ned siloer mellem development og operations - det er ikke længere "developers skriver kode og ops kører servere", men alle arbejder sammen om både kode og infrastruktur.

---

## 11. Sammenligning med Enterprise Best Practices

Lad mig nu sammenligne vores implementation med hvad man ville se i en enterprise organisation.

### Hvad Vi Følger

Vi følger mange industry best practices. Vi bruger multi-stage Docker builds, hvilket er standard for production workloads. Vi har health checks og restart policies på vores containers. Vi har resource constraints for at forhindre resource exhaustion. Vi har database migrations som kode, hvilket er standard i moderne applikationer. Vi har automatiseret CI/CD, hvilket er forventet i enhver DevOps organisation. Og vi har monitoring integreret fra start, ikke som en efterfølger.

Vi kører non-root containers, hvilket er en critical security best practice. Vi har automated dependency management, hvilket reducerer security debt. Og vi har infrastructure versioneret i Git, hvilket giver os audit trail og rollback capability.

### Områder til Forbedring

Men der er også områder hvor vi afviger fra enterprise standards. Vores secrets management er basic - en enterprise ville bruge dedikeret secrets management solutions med encryption, rotation og audit logging. Vi mangler infrastructure provisioning med Terraform - en enterprise ville have al cloud infrastructure defineret som kode. Vi bruger Docker Compose i production, hvor en enterprise ville bruge Kubernetes for scalability og resilience.

Vi har ingen explicit disaster recovery strategi, hvor en enterprise ville have automated backup og restore procedures defineret som kode. Vi mangler advanced deployment strategies som blue-green eller canary deployments. Og vi har ingen service mesh eller advanced networking policies.

### Hvad Vi Helt Mangler

Nogle enterprise capabilities mangler helt. Vi har ingen cloud-native infrastructure - vi deployer ikke til AWS, Azure eller GCP. Vi har ingen advanced orchestration capabilities som Kubernetes ville give os. Vi har ingen service mesh til advanced traffic management. Og vi har ingen multi-region replication for high availability.

Men det vigtige at huske er at enterprise capabilities kommer med enterprise complexity. For et projekt af denne størrelse - en student project eller en startup - er vores approach faktisk passende. Vi har balance mellem simplicity og functionality.

---

## 12. Skalerbarhed og Fremtiden

Lad mig diskutere hvordan dette system kunne skaleres hvis det skulle vokse.

### Nuværende Begrænsninger

I øjeblikket har vi et single-host deployment, hvor alle services kører på én server. Dette betyder at vores skalering er begrænset til vertical scaling - vi kan kun give serveren mere CPU og RAM. Vi har ingen horizontal scaling capability. Hvis load stiger, kan vi ikke automatisk spinde flere instances op af vores applikation.

Vi har ingen load balancer, så al traffic går direkte til vores ene server. Dette er en single point of failure - hvis serveren går ned, er hele applikationen nede. Og vi har ingen geographic distribution, så alle users har samme latency uanset hvor de er i verden.

### Migration til Production Scale

Hvis dette skulle blive et rigtigt production system med millioner af users, ville vi skulle migrere til en helt anden arkitektur. Først ville vi skulle adoptere Kubernetes til container orchestration. Dette ville give os horizontal pod autoscaling, så vi automatisk kunne spinde flere replicas op når load stiger.

Vi ville skulle implementere en load balancer til at distribuere traffic på tværs af multiple instances. Vi ville skulle migrere til managed database services som AWS RDS eller Google Cloud SQL, som giver automatisk backups, failover og read replicas. Vi ville tilføje en CDN foran vores static assets for at reducere load og latency.

Vi ville implementere auto-scaling policies defineret som kode, så infrastructure automatisk skalerer baseret på metrics. Vi ville tilføje health checks og circuit breakers for resilience. Og vi ville implementere blue-green eller canary deployment strategies for zero-downtime deployments.

Alt dette skulle være defineret som Infrastructure as Code - Terraform for cloud resources, Helm charts for Kubernetes deployments, og automated deployment pipelines. Men grundlaget vi har bygget med Docker, Compose og CI/CD ville stadig være værdifuldt - vi ville bare bygge ovenpå det.

### Vedligeholdelse og Operations

En af de store fordele ved vores nuværende setup er at vedligeholdelse er minimal. Dependency updates sker automatisk gennem Dependabot. Monitoring giver os alerts når noget går galt. Reproducible environments betyder at vi hurtigt kan recreate systemet hvis nødvendigt.

Men vi mangler automated testing af selve infrastrukturen. Vi tester vores applikations kode, men vi tester ikke vores infrastructure kode. I en mere mature setup ville vi have infrastructure tests der validerer at vores Terraform faktisk provisionerer korrekt, at vores Kubernetes deployments faktisk starter, og at vores monitoring faktisk fanger fejl.

---

## 13. Konklusion og Refleksioner

Lad mig nu konkludere min præsentation med nogle overordnede refleksioner.

### Projektets IaC Modenhed

Hvis jeg skulle vurdere dette projekts Infrastructure as Code modenhed på en skala fra et til fem, ville jeg give det en tre. Det er over halfway there, men der er stadig rum til forbedring.

På den positive side har vi solid foundation med Docker containerization og Docker Compose orchestration. Vi har excellent CI/CD implementation med automated testing, quality gates og deployment automation. Vi har database migrations som kode, hvilket mange projekter forsømmer. Vi har monitoring integreret fra start med configuration as code. Og vi har god separation mellem environments gennem environment variables.

Men vi mangler nogle key capabilities for at kalde det enterprise-grade. Vi mangler cloud infrastructure provisioning med tools som Terraform. Vi mangler advanced orchestration med Kubernetes. Vi mangler enterprise-grade secret management. Og vi mangler automated disaster recovery procedures.

### Læring og Udvikling

At bygge dette projekt har lært mig enormt meget om DevOps praksis. Jeg har lært værdien af at definere infrastructure som kode frem for at konfigurere ting manuelt. Jeg har set hvordan automation kan eliminere human error og accelerere deployment cycles. Jeg har erfaret hvordan version control af infrastructure giver traceability og rollback capability.

Jeg har også lært at Infrastructure as Code ikke er gratis - det kræver initial investment i at lære tools og skrive konfiguration. Men den investment betaler sig tilbage i reproducibility, reliability og velocity.

### Hvad Ville Jeg Gøre Anderledes?

Hvis jeg skulle starte projektet forfra med hvad jeg ved nu, ville jeg gøre nogle ting anderledes. Jeg ville fra start have implementeret proper secrets management i stedet for at bruge .env filer. Jeg ville have investeret i Terraform til server provisioning i stedet for at gøre det manuelt. Og jeg ville have skrevet infrastructure tests til at validere mine configurations.

Men jeg ville også være forsigtig med ikke at over-engineer. Det er fristende at hoppe direkte til Kubernetes og service meshes og alle de fancy tools, men for dette projekts størrelse ville det være overkill. Sometimes simplicity is a feature, not a bug.

### Relevans for Erhvervslivet

Hvad jeg har bygget her er direkte anvendeligt i erhvervslivet. De fleste moderne teknologi-virksomheder bruger Docker containerization. Mange bruger Docker Compose til smaller deployments eller development environments. Næsten alle bruger CI/CD pipelines. Database migrations som kode er industry standard. Og monitoring as code er becoming standard practice.

De værktøjer og principper jeg har demonstreret her - declarative configuration, version control af infrastructure, automated deployment, infrastructure testing - det er kernen i moderne DevOps praksis. En nyuddannet der kan demonstrere forståelse af disse koncepter er valuable til en organisation.

### Fremtidige Forbedringer

Hvis jeg skulle fortsætte udviklingen af dette projekt, ville mine næste steps være at implementere Terraform for infrastructure provisioning, så jeg kan spin up hele mit deployment target automatisk. Jeg ville migrere til Kubernetes for bedre skalering og resilience. Jeg ville implementere proper secrets management med HashiCorp Vault. Og jeg ville tilføje infrastructure tests til min CI pipeline.

Men vigtigst af alt ville jeg fokusere på documentation. Infrastructure as Code er self-documenting til en vis grad, men det hjælper stadig at have architecture diagrams og runbooks der forklarer hvordan systemet hænger sammen og hvordan man troubleshooter almindelige problemer.

### Afsluttende Tanker

Infrastructure as Code er ikke bare en teknisk praksis - det er en kulturel shift. Det handler om at behandle infrastructure med samme respekt og disciplin som vi behandler application code. Det handler om at bryde ned siloer mellem development og operations. Og det handler om at bygge systemer der er reproducible, testable og maintainable.

Dette projekt demonstrerer at selv et relativt simpelt setup kan drage nytte af IaC principper. Vi behøver ikke kompleks cloud infrastructure eller Kubernetes clusters for at få værdi ud af at definere vores infrastructure som kode. Bare det faktum at jeg kan clone dette repository og have hele stacken kørende med én kommando er enormt værdifuldt.

DevOps handler fundamentalt om at reducere friction mellem at skrive kode og at køre den i production. Infrastructure as Code er et af de mest kraftfulde værktøjer vi har til at reducere den friction. Ved at definere alt som kode, versionere det i Git, og automatisere deployment, opnår vi hastighed uden at ofre stabilitet.

Tak for jeres opmærksomhed. Jeg vil meget gerne besvare eventuelle spørgsmål I måtte have.

---

## Mulige Eksamensspørgsmål og Svar

Lad mig også forberede nogle potentielle spørgsmål I måtte stille.

**Spørgsmål: Hvorfor valgte du Docker Compose fremfor Kubernetes?**

Det er et godt spørgsmål. Valget mellem Docker Compose og Kubernetes handler om trade-offs mellem simplicity og capabilities. For dette projekt, som er en single-host deployment med relativt lav traffic, giver Docker Compose os hvad vi behøver uden at tilføje Kubernetes' betydelige complexity. Kubernetes ville give os horizontal scaling, automatic failover og advanced health checks, men det ville også kræve at vi skulle lære et helt nyt ecosystem af tools og koncepter. For en startup eller et mindre projekt er Docker Compose ofte det rigtige valg. Men hvis dette skulle skalere til millioner af users, ville vi definitivt skulle migrere til Kubernetes.

**Spørgsmål: Hvordan håndterer du rollback hvis en deployment går galt?**

Det er faktisk et område hvor vi kunne være bedre. I øjeblikket hvis en deployment fejler, ville vi manuelt skulle SSH til serveren og køre "docker-compose up" med en tidligere version af vores image. Vi har alle tidligere images i vores container registry, så det er muligt, men det er ikke automatiseret. I en mere mature setup ville vi have automated rollback som en del af vores deployment pipeline - hvis health checks fejler efter deployment, ruller systemet automatisk tilbage til previous version. Det er noget jeg ville tilføje hvis dette var et rigtigt production system.

**Spørgsmål: Hvordan sikrer du at secrets ikke kommer i Git?**

Vi bruger flere lag af beskyttelse. For det første har vi .env filen i vores .gitignore, så den bliver aldrig commitet. For det andet bruger vi GitHub secrets til sensitive data i vores CI/CD pipeline, så de aldrig er synlige i workflow logs. For det tredje scanner vi vores repository periodisk for accidentally committed secrets. Men jeg vil indrømme at det ikke er perfect - i en enterprise setting ville vi bruge en dedikeret secrets management solution som HashiCorp Vault, som giver encryption at rest, automatic rotation og audit logging.

**Spørgsmål: Hvad er fordelen ved multi-stage Docker builds?**

Multi-stage builds giver os flere fordele. For det første får vi meget mindre images, fordi vi ikke inkluderer build tools i vores final image - kun runtime dependencies. Et golang builder image kan være hundredvis af megabytes, mens vores final Alpine-baserede image er under tyve megabytes. Det betyder hurtigere pulls når vi deployer. For det andet forbedrer det security, fordi vi har færre packages i vores runtime image, hvilket betyder mindre attack surface. Og for det tredje optimerer det layer caching i vores CI/CD pipeline, hvilket gør builds hurtigere.

**Spørgsmål: Hvordan ville du implementere zero-downtime deployments?**

Det kræver flere ting. For det første skulle vi have load balancing og multiple replicas af vores applikation. Så skulle vi implementere en rolling update strategy, hvor vi opdaterer én replica ad gangen og venter på at den er healthy før vi går videre til den næste. Eller vi kunne implementere blue-green deployment, hvor vi spinner en helt ny version op ved siden af den gamle, shifter traffic over, og kun tager den gamle ned når den nye er verified healthy. Med Docker Compose alene er det svært at opnå true zero-downtime deployments - det er et område hvor Kubernetes virkelig skinner, fordi det har disse strategies built in.

**Spørgsmål: Hvordan tester du dine infrastructure changes?**

Det er faktisk et område hvor vi kunne være meget bedre. I øjeblikket tester vi vores application code grundigt gennem unit tests og integration tests i vores CI pipeline. Men vi tester ikke virkelig vores infrastructure code. Ideelt set skulle vi have infrastructure tests der validerer at vores Docker Compose konfiguration faktisk starter korrekt, at services kan kommunikere med hinanden, og at monitoring faktisk fanger fejl. Vi kunne bruge tools som Terratest eller Testcontainers til at skrive sådanne tests. Det ville give os meget større confidence når vi laver infrastructure changes.

---

## Ordliste - Danske DevOps Termer

Til sidst vil jeg lige give en oversigt over vigtige termer, da eksamen er på dansk:

Infrastructure as Code betyder infrastruktur som kode - princippet om at definere infrastruktur gennem versionerede konfigurationsfiler frem for manuel konfiguration. Container Orchestration er container orkestrering - automatisk håndtering af deployment, scaling og management af containers. Declarative Configuration betyder deklarativ konfiguration - at beskrive ønsket tilstand frem for at specificere skridt til at nå den tilstand.

Multi-stage Build er en flertrins byggeproces - Docker build strategi der bruger multiple FROM statements til at optimere image størrelse. Continuous Integration og Continuous Deployment betyder kontinuerlig integration og kontinuerlig udrulning - praksis om at automatisk teste og deploye kode ændringer. Database Migration er database-migrering - versionerede scripts der ændrer database schema.

Secret Management betyder hemmelighedshåndtering - sikker håndtering af passwords, API keys og andre sensitive data. Resource Constraints er ressourcebegrænsninger - limits på CPU og memory for containers. Health Checks betyder sundhedstjek - automatiske checks for at verificere at services kører korrekt. Persistent Storage er vedvarende lagring - data der overlever container restarts.

Observability betyder observerbarhed - evnen til at forstå system state gennem metrics, logs og traces. Provisioning betyder klargøring - processen med at sætte infrastruktur op. Rollback er tilbagerulning - at gå tilbage til en tidligere version. Og Idempotency er idempotens - egenskaben at en operation giver samme resultat uanset hvor mange gange den køres.

Med disse begreber skulle vi være godt rustet til at diskutere Infrastructure as Code på dansk.

---

*Dette dokument er forberedt som mundtlig præsentation til DevOps eksamen og giver en detaljeret gennemgang af Infrastructure as Code implementation i GoSearch-projektet.*
