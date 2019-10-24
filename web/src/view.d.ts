declare namespace OpenAPI2 {
  export interface webviewView {
    log?: string;
    resources?: webviewResource[];
    logTimestamps?: boolean;
    featureFlags?: object;
    needAnalyticsNudge?: boolean;
    runningTiltBuild?: webviewTiltBuild;
    latestTiltBuild?: webviewTiltBuild;
    tiltCloudUsername?: string;
    tiltCloudSchemeHost?: string;
    tiltCloudTeamID?: string;
    fatalError?: string;
  }
  export interface webviewTiltBuild {
    version?: string;
    commitSHA?: string;
    date?: string;
    dev?: boolean;
  }
  export interface webviewResource {}
}
