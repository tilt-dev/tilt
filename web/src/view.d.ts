declare namespace Proto {
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
    DEPRECATEDLatestTiltBuild?: webviewTiltBuild;
    suggestedTiltVersion?: string;
    versionSettings?: webviewVersionSettings;
    tiltCloudUsername?: string;
    tiltCloudTeamName?: string;
    tiltCloudSchemeHost?: string;
    tiltCloudTeamID?: string;
    fatalError?: string;
    logList?: webviewLogList;
    /**
     * Allows us to synchronize on a running Tilt intance,
     * so we can tell when Tilt restarted.
     */
    tiltStartTime?: string;
    tiltfileKey?: string;
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
  export interface webviewTargetSpec {
    id?: string;
    type?: string;
    hasLiveUpdate?: boolean;
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
    lastDeployTime?: string;
    triggerMode?: number;
    buildHistory?: webviewBuildRecord[];
    currentBuild?: webviewBuildRecord;
    pendingBuildSince?: string;
    hasPendingChanges?: boolean;
    endpointLinks?: webviewLink[];
    podID?: string;
    k8sResourceInfo?: webviewK8sResourceInfo;
    localResourceInfo?: webviewLocalResourceInfo;
    runtimeStatus?: string;
    updateStatus?: string;
    isTiltfile?: boolean;
    specs?: webviewTargetSpec[];
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
    /**
     * When we store warnings in the LogStore, we break them up into lines and
     * store them as a series of line segments. 'anchor' marks the beginning of a
     * series of logs that should be kept together.
     *
     * Anchor warning1, line1
     *        warning1, line2
     * Anchor warning2, line1
     */
    anchor?: boolean;
    /**
     * Context-specific optional fields for a log segment.
     * Used for experimenting with new types of log metadata.
     */
    fields?: object;
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
  export interface webviewLocalResourceInfo {
    pid?: string;
    isTest?: boolean;
  }
  export interface webviewLink {
    url?: string;
    name?: string;
  }
  export interface webviewK8sResourceInfo {
    podName?: string;
    podCreationTime?: string;
    podUpdateStartTime?: string;
    podStatus?: string;
    podStatusMessage?: string;
    allContainersReady?: boolean;
    podRestarts?: number;
    spanId?: string;
    displayNames?: string[];
  }
  export interface webviewBuildRecord {
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
