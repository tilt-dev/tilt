declare namespace OpenAPI2 {
  export interface webviewYAMLResourceInfo {
    k8sResources?: string[];
  }
  export interface webviewView {
    log?: string;
    resources?: webviewResource[];
    logTimestamps?: boolean;
    featureFlags?: object;
    needsAnalyticsNudge?: boolean;
    runningTiltBuild?: webviewTiltBuild;
    latestTiltBuild?: webviewTiltBuild;
    tiltCloudUsername?: string;
    tiltCloudSchemeHost?: string;
    tiltCloudTeamID?: string;
    fatalError?: string;
  }
  export interface webviewUploadSnapshotResponse {
    url?: string;
  }
  export interface webviewTiltBuild {
    version?: string;
    commitSHA?: string;
    date?: string;
    dev?: boolean;
  }
  export interface webviewSnapshotHighlight {
    beginningLogId?: string;
    endingLogId?: string;
    text?: string;
  }
  export interface webviewSnapshot {
    view?: webviewView;
    isSidebarClosed?: boolean;
    path?: string;
    snapshotHighlight?: webviewSnapshotHighlight;
  }
  export interface webviewResource {
    name?: string;
    directoriesWatched?: string[];
    pathsWatched?: string[];
    lastDeployTime?: string;
    triggerMode?: number;
    buildHistory?: webviewBuildRecord[];
    currentBuild?: webviewBuildRecord;
    pendingBuildReason?: number;
    pendingBuildEdits?: string[];
    pendingBuildSince?: string;
    hasPendingChanges?: boolean;
    endpoints?: string[];
    podID?: string;
    k8sResourceInfo?: webviewK8sResourceInfo;
    dcResourceInfo?: webviewDCResourceInfo;
    yamlResourceInfo?: webviewYAMLResourceInfo;
    localResourceInfo?: webviewLocalResourceInfo;
    runtimeStatus?: string;
    isTiltfile?: boolean;
    showBuildStatus?: boolean;
    combinedLog?: string;
    crashLog?: string;
    alerts?: webviewAlert[];
    facets?: webviewFacet[];
  }
  export interface webviewLocalResourceInfo {}
  export interface webviewK8sResourceInfo {
    podName?: string;
    podCreationTime?: string;
    podUpdateStartTime?: string;
    podStatus?: string;
    podStatusMessage?: string;
    allContainersReady?: boolean;
    podRestarts?: number;
    podLog?: string;
  }
  export interface webviewFacet {
    name?: string;
    value?: string;
  }
  export interface webviewDCResourceInfo {
    configPaths?: string[];
    containerStatus?: string;
    containerID?: string;
    log?: string;
    startTime?: string;
  }
  export interface webviewBuildRecord {
    edits?: string[];
    error?: string;
    warnings?: string[];
    startTime?: string;
    finishTime?: string;
    log?: string;
    isCrashRebuild?: boolean;
  }
  export interface webviewAlert {
    alertType?: string;
    header?: string;
    message?: string;
    timestamp?: string;
    resourceName?: string;
  }
}
