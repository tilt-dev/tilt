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
    /**
     * Allows us to synchronize on a running Tilt intance,
     * so we can tell when Tilt restarted.
     */
    tiltStartTime?: string;
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
    crashLog?: string;
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
    /**
     * [from_checkpoint, to_checkpoint)
     *
     * An interval of [0, 0) means that the server isn't using
     * the incremental load protocol.
     *
     * An interval of [-1, -1) means that the server doesn't have new logs
     * to send down.
     */
    fromCheckpoint?: number;
    toCheckpoint?: number;
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
    spanId?: string;
  }
  export interface webviewFacet {
    name?: string;
    value?: string;
  }
  export interface webviewDCResourceInfo {
    configPaths?: string[];
    containerStatus?: string;
    containerID?: string;
    startTime?: string;
    spanId?: string;
  }
  export interface webviewBuildRecord {
    edits?: string[];
    error?: string;
    warnings?: string[];
    startTime?: string;
    finishTime?: string;
    isCrashRebuild?: boolean;
    /**
     * The span id for this build record's logs in the main logstore.
     */
    spanId?: string;
  }
  export interface webviewAckWebsocketResponse {}
  export interface webviewAckWebsocketRequest {
    toCheckpoint?: number;
    /**
     * Allows us to synchronize on a running Tilt intance,
     * so we can tell when we're talking to the same Tilt.
     */
    tiltStartTime?: string;
  }
}
