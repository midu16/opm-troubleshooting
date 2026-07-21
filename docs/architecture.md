# Software Architecture

## System Overview

```mermaid
graph TB
    subgraph Binaries["CLI Binaries"]
        OPM[opm-diagnose]
        CBI[catalog-bundle-inspect]
        TD[telco-diagnose]
        BV[batch-validate]
        RAGS[ocp-rag-server]
        RAGI[ocp-rag-ingest]
    end

    subgraph CLI["CLI Layer"]
        RunDiagnose["cli.RunDiagnose()"]
        Run["cli.Run()"]
        RunTelco["cli.RunTelcoDiagnose()"]
    end

    OPM --> RunDiagnose
    CBI --> Run
    TD --> RunTelco
    BV --> WF

    RunDiagnose --> AN
    Run --> AN
    Run --> WF
    RunTelco --> AN

    subgraph Input["Data Input Layer"]
        MG["mustgather\n(YAML parser)"]
        DS["datasource\n(ClusterDataSource\ninterface)"]
        LC["livecluster\n(Kubernetes API)"]
    end

    DS --- MG
    LC --> DS

    subgraph OLM["OLM Catalog & Bundle Layer"]
        CAT["catalog\n(FBC render +\nchannel resolution)"]
        WF["workflow\n(inspect pipeline)"]
        II["imageinspect\n(image label\nextraction)"]
        CSV["bundlecsv\n(CSV annotation\nparsing)"]
    end

    WF --> CAT
    WF --> II
    II --> CSV

    subgraph AI["AI & Analysis Layer"]
        CL["claudeapi\n(Anthropic API)"]
        ADHD["adhd\n(13-frame divergent\nanalysis engine)"]
        RAG["rag.Engine\n(embedded Go RAG\nwith chromem-go)"]
    end

    ADHD --> CL

    subgraph Health["Health Check & Noise Layer"]
        HC["healthcheck\n(20 OLM + 13 infra\ndimensions)"]
        NF["noise\n(environment-aware\nfiltering)"]
    end

    NF --> HC

    subgraph SCC["Source Code Correlation Layer"]
        OS["openshift\n(repo resolver +\nclassifier + grading)"]
        GD["gitdelta\n(commit diff)"]
        CA["codeanalysis\n(pattern search)"]
    end

    OS --> CA
    OS --> GD
    OS --> II

    subgraph Output["Output & Reporting Layer"]
        RCA["rca\n(14-section Markdown\nRCA report)"]
    end

    subgraph Persist["Persistence & Learning Layer"]
        SS["session\n(JSON on disk)"]
        MD["metadata\n(SQLite store)"]
        LN["learning\n(symptom matching +\nframe boost)"]
    end

    MD --> SS
    LN --> MD

    subgraph Domain["Domain Configuration"]
        TL["telco\n(27 operator profiles)"]
    end

    subgraph Core["Central Orchestrator"]
        AN["analysis\n(12-step pipeline)"]
    end

    AN --> MG
    AN --> DS
    AN --> CAT
    AN --> II
    AN --> WF
    AN --> CL
    AN --> ADHD
    AN --> HC
    AN --> NF
    AN --> OS
    AN --> GD
    AN --> CA
    AN --> RCA
    AN --> SS
    AN --> MD
    AN --> LN
    AN --> TL
    AN --> RAG

    RCA --> ADHD
    RCA --> HC
    RCA --> NF
    RCA --> SS

    HC --> DS
    HC --> TL

    ADHD --> DS

    LN --> ADHD
    LN --> HC

    subgraph RAGStore["RAG Vector Store (embedded)"]
        CHROMEM[("chromem-go\nIn-Process DB")]
        C1["ocp_docs"]
        C2["operator_code"]
        C3["telco_configs"]
        C4["known_issues"]
        C5["manifests"]
        CHROMEM --- C1
        CHROMEM --- C2
        CHROMEM --- C3
        CHROMEM --- C4
        CHROMEM --- C5
    end

    RAG --> CHROMEM

    subgraph External["External Systems"]
        K8S[("Kubernetes\nAPI Server")]
        REG[("Container\nRegistries")]
        GH[("GitHub\nAPI")]
        GIT[("Git Repos\n(clone)")]
        ANTH[("Anthropic\nClaude API")]
        SQL[("SQLite\nDatabase")]
        RHDOCS[("Red Hat\nDocs")]
        OLLAMA[("Ollama\nEmbeddings")]
    end

    LC -.-> K8S
    II -.-> REG
    CAT -.-> REG
    CSV -.-> REG
    OS -.-> GH
    OS -.-> GIT
    GD -.-> GIT
    CA -.-> GIT
    CL -.-> ANTH
    MD -.-> SQL
    RAG -.-> RHDOCS
    RAG -.-> OLLAMA

    classDef binary fill:#4a90d9,stroke:#2c5f8a,color:#fff
    classDef core fill:#e8534a,stroke:#b33c33,color:#fff
    classDef input fill:#50b848,stroke:#377d32,color:#fff
    classDef olm fill:#f5a623,stroke:#c7841a,color:#fff
    classDef ai fill:#9b59b6,stroke:#7d3c98,color:#fff
    classDef health fill:#1abc9c,stroke:#148f77,color:#fff
    classDef scc fill:#e67e22,stroke:#b8651c,color:#fff
    classDef output fill:#3498db,stroke:#2471a3,color:#fff
    classDef persist fill:#7f8c8d,stroke:#5d6d7e,color:#fff
    classDef domain fill:#95a5a6,stroke:#717d7e,color:#fff
    classDef rag fill:#e91e63,stroke:#c2185b,color:#fff
    classDef ext fill:#2c3e50,stroke:#1a252f,color:#fff

    RAGS --> RAG
    RAGI --> RAG

    class OPM,CBI,TD,BV,RAGS,RAGI binary
    class AN core
    class MG,DS,LC input
    class CAT,WF,II,CSV olm
    class CL,ADHD ai
    class HC,NF health
    class OS,GD,CA scc
    class RCA output
    class SS,MD,LN persist
    class TL domain
    class RAG,CHROMEM,C1,C2,C3,C4,C5 rag
    class K8S,REG,GH,GIT,ANTH,SQL,RHDOCS ext
```

## Analysis Pipeline (12-step must-gather flow)

```mermaid
flowchart TD
    START([Must-Gather or Live Cluster]) --> PARSE

    subgraph Input["Data Collection"]
        PARSE["Parse operator state\n(mustgather / livecluster)"]
        RENDER["Render FBC catalog"]
    end

    PARSE --> S0
    RENDER --> S2

    subgraph Pipeline["Per-Operator 12-Step Analysis"]
        S0["Step 0: RCA Pattern Detection\n(13 failure patterns)"]
        S1["Step 1: OLM Health Check\n(20 dimensions)"]
        S2["Step 2: Bundle Metadata\n(commit, version, URL)"]
        S3["Step 3: Git Delta\n(diff installed vs target)"]
        S4["Step 4: Code Analysis\n(search source for failure strings)"]
        S5["Step 5: Claude AI Analysis\n(fault analysis prompt)"]
        S5_5["Step 5.5: RAG Enrichment\n(knowledge base lookup)"]
        S6["Step 6: Infrastructure Health\n(13 dimensions)"]
        S7["Step 7: ADHD Divergent Analysis\n(13 frames, 4 phases)"]
        S8["Step 8: Noise Filtering\n(environment-aware)"]
        S9["Step 9: Repo Correlation\n(resolve + verify + classify + grade)"]
        S10["Step 10: Learning Lookup\n(symptom fingerprint matching)"]
        S11["Step 11: Metadata Recording\n(persist to SQLite)"]
    end

    S0 --> S1 --> S2 --> S3 --> S4 --> S5 --> S5_5 --> S6 --> S7 --> S8 --> S9 --> S10 --> S11

    S11 --> S12["Step 12: RCA Report Generation\n(14-section Markdown)"]

    S12 --> OUTPUT([RCA Document])

    classDef step fill:#3498db,stroke:#2471a3,color:#fff
    classDef io fill:#2ecc71,stroke:#27ae60,color:#fff
    classDef rag fill:#e91e63,stroke:#c2185b,color:#fff
    class S0,S1,S2,S3,S4,S5,S6,S7,S8,S9,S10,S11,S12 step
    class S5_5 rag
    class START,OUTPUT,PARSE,RENDER io
```

## Source Code Correlation Pipeline (Step 9 detail)

```mermaid
flowchart LR
    ENTRY([Operator +\nFailure Reason]) --> CASCADE

    subgraph CASCADE["Repo Resolution Cascade"]
        direction TB
        R1["1. Static Registry\n(40+ mappings)"]
        R2["2. Bundle Image Labels\n(io.openshift.build.*)"]
        R3["3. Bundle CSV Annotations\n(repository URLs)"]
        R4["4. Package Name Inference\n(openshift/{name})"]
        R1 -->|not found| R2
        R2 -->|not found| R3
        R3 -->|not found| R4
    end

    CASCADE --> VERIFY["GitHub API\nVerification"]

    VERIFY --> CLONE{Commit\navailable?}
    CLONE -->|yes| PINNED["CloneAndAnalyze\nat deployed commit"]
    CLONE -->|no| HEAD["CloneOrUpdate\nat HEAD"]

    PINNED --> CLASSIFY
    HEAD --> CLASSIFY

    subgraph CLASSIFY["Enhanced Classification"]
        direction TB
        CW["Keyword Score\n(weight: 0.40)"]
        CM["Code Match Score\n(weight: 0.30)"]
        CD["Delta Evidence\n(weight: 0.20)"]
        CP["Pin Bonus\n(weight: 0.10)"]
    end

    CLASSIFY --> GRADE["Confidence Grade\nA / B / C / D / F"]
    GRADE --> RESULT(["Classification:\ncode_bug | configuration |\ninfrastructure | unknown\n+ grade + evidence"])

    classDef cascade fill:#f39c12,stroke:#d68910,color:#fff
    classDef verify fill:#27ae60,stroke:#1e8449,color:#fff
    classDef classify fill:#8e44ad,stroke:#6c3483,color:#fff
    class R1,R2,R3,R4 cascade
    class VERIFY verify
    class CW,CM,CD,CP classify
```

## ADHD Divergent Analysis Engine

```mermaid
flowchart TD
    PROBLEM([Problem Statement +\nSymptoms + Cluster Snapshot]) --> DIVERGE

    subgraph DIVERGE["Phase 1: Diverge"]
        direction LR
        F1["Network\nEngineer"]
        F2["Storage\nAdmin"]
        F3["Security\nAuditor"]
        F4["Capacity\nPlanner"]
        F5["Upgrade\nSpecialist"]
        F6["3AM\nOn-Call SRE"]
        F7["etcd\nSpecialist"]
        F8["Adversarial\nThinker"]
        F9["Assumption\nRemover"]
        F10["Dependency\nWalker"]
        F11["Timeline\nReconstructor"]
        F12["Platform\nArchitect"]
        F13["Source Code\nForensics"]
    end

    DIVERGE --> SCORE

    subgraph SCORE["Phase 2: Score"]
        direction TB
        LIE["Likelihood x 0.40"]
        IMP["Impact x 0.25"]
        EVI["Evidence x 0.35"]
        LIE --> TOTAL["Composite Score"]
        IMP --> TOTAL
        EVI --> TOTAL
        TOTAL --> TRAP{"Trap\nDetection\n(7 patterns)"}
    end

    TRAP -->|trap| FLAGGED["Flagged as trap\n(looks-like-cause\nbut is symptom)"]
    TRAP -->|clean| CLUSTER_PHASE

    subgraph CLUSTER_PHASE["Phase 3: Cluster"]
        SHORTLIST["Top-K Hypotheses\nranked by score"]
        NONOBVIOUS["Non-Obvious Finding\n(from 'wild' frame)"]
    end

    CLUSTER_PHASE --> DEEPEN

    subgraph DEEPEN["Phase 4: Deepen"]
        SKETCH["Investigation Sketch"]
        RISK["Load-Bearing Risk"]
        STEP1["Recommended First Step"]
        CHILDREN["Child Hypotheses"]
    end

    DEEPEN --> RESULT([ADHD Diagnosis Result])

    classDef frame fill:#e74c3c,stroke:#c0392b,color:#fff
    classDef score fill:#3498db,stroke:#2471a3,color:#fff
    classDef cluster fill:#2ecc71,stroke:#27ae60,color:#fff
    classDef deep fill:#9b59b6,stroke:#7d3c98,color:#fff
    class F1,F2,F3,F4,F5,F6,F7,F8,F9,F10,F11,F12,F13 frame
    class LIE,IMP,EVI,TOTAL,TRAP score
    class SHORTLIST,NONOBVIOUS cluster
    class SKETCH,RISK,STEP1,CHILDREN deep
```

## RAG Knowledge Base (Step 5.5 detail)

```mermaid
flowchart LR
    subgraph Go["Go Analysis Pipeline"]
        ENGINE["rag.Engine\n(direct Go API)"]
        HYBRID["hybridRetrieve()\n(semantic + keyword)"]
        ENGINE --> HYBRID
    end

    HYBRID --> CHROMEM

    subgraph Store["chromem-go (in-process, embedded)"]
        CHROMEM[("chromem-go\nPersistent DB")]
        C1["ocp_docs\n(OCP 4.22 docs)"]
        C2["operator_code\n(Go source via go/ast)"]
        C3["telco_configs\n(reference configs)"]
        C4["known_issues\n(errata/bugs)"]
        C5["manifests\n(CRDs/YAML)"]
        CHROMEM --- C1
        CHROMEM --- C2
        CHROMEM --- C3
        CHROMEM --- C4
        CHROMEM --- C5
    end

    subgraph Embed["Embedding"]
        OLLAMA["Ollama HTTP API\n/api/embed\nqwen3-embedding:latest"]
    end

    HYBRID --> OLLAMA

    subgraph Sources["Data Sources (ingested offline)"]
        RH["docs.redhat.com\nOCP 4.22"]
        GH["github.com/openshift/*\n(27 repos)"]
        TELCO["openshift-kni/\ntelco-reference\nrelease-4.22"]
    end

    RH -.->|"scrape + chunk"| C1
    GH -.->|"clone + go/parser"| C2
    TELCO -.->|"clone + load YAML"| C3

    subgraph MCP["Optional: Standalone MCP Server"]
        SRV["ocp-rag-server\n(stdio transport)"]
        T1["search_docs"]
        T2["search_operator_code"]
        T3["search_telco_configs"]
        T4["troubleshoot_operator"]
        T5["get_operator_info"]
        T6["search_known_issues"]
        T7["search_errata"]
        T8["update_rag"]
        SRV --> T1 & T2 & T3 & T4 & T5 & T6 & T7 & T8
    end

    SRV --> ENGINE

    classDef go fill:#3498db,stroke:#2471a3,color:#fff
    classDef store fill:#2c3e50,stroke:#1a252f,color:#fff
    classDef source fill:#27ae60,stroke:#1e8449,color:#fff
    classDef embed fill:#e91e63,stroke:#c2185b,color:#fff
    classDef mcp fill:#9b59b6,stroke:#7d3c98,color:#fff
    class ENGINE,HYBRID go
    class CHROMEM,C1,C2,C3,C4,C5 store
    class RH,GH,TELCO source
    class OLLAMA embed
    class SRV,T1,T2,T3,T4,T5,T6,T7,T8 mcp
```

## Data Source Abstraction

```mermaid
flowchart LR
    subgraph Interface["ClusterDataSource Interface\n(20+ methods)"]
        direction TB
        OLM["OLM Resources\nSubscriptions, CSVs,\nInstallPlans, CatalogSources"]
        WORK["Workloads\nDeployments, Pods,\nEvents"]
        INFRA["Infrastructure\nNodes, PVs, MCPs,\nClusterVersion"]
        NET["Networking\nNetwork config,\nRoutes, DNS"]
        SEC["Security\nSecrets, ConfigMaps,\nClusterOperators"]
    end

    subgraph Impl["Implementations"]
        MGS["MustGatherSource\n(offline YAML files)"]
        LCS["LiveClusterSource\n(Kubernetes API)"]
    end

    MGS --> Interface
    LCS --> Interface

    Interface --> HC["healthcheck"]
    Interface --> ADHD_PKG["adhd"]

    MG_DIR[("Must-Gather\nDirectory")] -.-> MGS
    K8S[("Kubernetes\nAPI Server")] -.-> LCS

    classDef iface fill:#34495e,stroke:#2c3e50,color:#fff
    classDef impl fill:#16a085,stroke:#1abc9c,color:#fff
    class OLM,WORK,INFRA,NET,SEC iface
    class MGS,LCS impl
```

## Self-Learning Feedback Loop

```mermaid
flowchart LR
    RUN["Analysis Run\n(operator + health +\nnoise + patterns)"] --> FP["BuildFingerprint()\n(symptom hash)"]

    FP --> STORE[("SQLite\nMetadataStore")]

    STORE --> |"next run"| FIND["FindSimilarIssues()\n3-tier matching"]

    subgraph MATCH["Similarity Matching"]
        direction TB
        T1["Tier 1: Exact hash match"]
        T2["Tier 2: Operator-scoped\nsimilarity"]
        T3["Tier 3: Global\nsimilarity"]
    end

    FIND --> MATCH

    STORE --> BOOST["ComputeBoostFactors()\n(per-frame accuracy)"]
    BOOST --> APPLY["ApplyBoosts()\n(adjust ADHD scores)"]

    MATCH --> RCA_OUT["RCA Report:\nSimilar Issues section"]
    APPLY --> ADHD_OUT["ADHD Engine:\nboosted hypothesis scores"]

    classDef store fill:#2c3e50,stroke:#1a252f,color:#fff
    classDef process fill:#2980b9,stroke:#1f618d,color:#fff
    class STORE store
    class FP,FIND,BOOST,APPLY process
```

## Package Dependency Graph

```mermaid
graph TD
    subgraph Leaf["Leaf Packages (no internal deps)"]
        bundlecsv
        catalog
        codeanalysis
        gitdelta
        mustgather
        session
        telco
        testfixture
        claudeapi
        rag["rag\n(embedded RAG engine\n+ chromem-go)"]
    end

    subgraph Mid["Mid-Level Packages"]
        imageinspect --> bundlecsv
        datasource --> mustgather
        livecluster --> datasource
        workflow --> catalog
        workflow --> imageinspect
        noise --> healthcheck
        metadata --> session
    end

    subgraph High["High-Level Packages"]
        healthcheck --> datasource
        healthcheck --> mustgather
        healthcheck --> telco
        adhd --> claudeapi
        adhd --> datasource
        openshift --> codeanalysis
        openshift --> gitdelta
        openshift --> healthcheck
        openshift --> imageinspect
        learning --> adhd
        learning --> healthcheck
        learning --> metadata
        learning --> mustgather
        learning --> noise
        learning --> rca_pkg["rca"]
    end

    subgraph Top["Top-Level Packages"]
        rca_pkg --> adhd
        rca_pkg --> claudeapi
        rca_pkg --> codeanalysis
        rca_pkg --> gitdelta
        rca_pkg --> healthcheck
        rca_pkg --> imageinspect
        rca_pkg --> mustgather
        rca_pkg --> noise
        rca_pkg --> session
        analysis --> adhd
        analysis --> catalog
        analysis --> claudeapi
        analysis --> codeanalysis
        analysis --> datasource
        analysis --> gitdelta
        analysis --> healthcheck
        analysis --> imageinspect
        analysis --> learning
        analysis --> metadata
        analysis --> mustgather
        analysis --> noise
        analysis --> openshift
        analysis --> rca_pkg
        analysis --> session
        analysis --> telco
        analysis --> rag
        analysis --> workflow
    end

    classDef leaf fill:#27ae60,stroke:#1e8449,color:#fff
    classDef mid fill:#f39c12,stroke:#d68910,color:#fff
    classDef high fill:#e74c3c,stroke:#c0392b,color:#fff
    classDef top fill:#8e44ad,stroke:#6c3483,color:#fff

    class bundlecsv,catalog,codeanalysis,gitdelta,mustgather,session,telco,testfixture,claudeapi,rag leaf
    class imageinspect,datasource,livecluster,workflow,noise,metadata mid
    class healthcheck,adhd,openshift,learning high
    class rca_pkg,analysis top
```
