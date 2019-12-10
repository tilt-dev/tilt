declare namespace Proto {
  export interface webviewYAMLResourceInfo {
    k8sResources?: string[];
  }
  export interface webviewView {
    log?: string;
    resources?: webviewResource[];
    /**
     * We used to have a setting that allowed users to dynamically
     * prepend timestamps in logs.
     */
    DEPRECATEDLogTimestamps?: boolean;
    featureFlags?: object;
    needsAnalyticsNudge?: boolean;
    runningTiltBuild?: webviewTiltBuild;
    latestTiltBuild?: webviewTiltBuild;
    versionSettings?: webviewVersionSettings;
    tiltCloudUsername?: string;
    tiltCloudSchemeHost?: string;
    tiltCloudTeamID?: string;
    fatalError?: string;
    logList?: webviewLogList;
  }
  export interface webviewVersionSettings {
    checkUpdates?: boolean;
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
    beginningLogID?: string;
    endingLogID?: string;
    text?: string;
  }
  export interface webviewSnapshot {
    view?: webviewView;
    isSidebarClosed?: boolean;
    path?: string;
    snapshotHighlight?: webviewSnapshotHighlight;
    snapshotLink?: string;
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
    queued?: boolean;
  }
  export interface webviewLogSpan {
    manifestName?: string;
  }
  export interface webviewLogSegment {
    spanId?: string;
    time?: string;
    text?: string;
    level?: string;
  }
  export interface webviewLogList {
    spans?: object;
    segments?: webviewLogSegment[];
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
