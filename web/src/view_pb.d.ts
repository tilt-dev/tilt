import * as jspb from "google-protobuf"

import * as google_api_annotations_pb from '../../google/api/annotations_pb';

export class Resource extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Resource.AsObject;
  static toObject(includeInstance: boolean, msg: Resource): Resource.AsObject;
  static serializeBinaryToWriter(message: Resource, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Resource;
  static deserializeBinaryFromReader(message: Resource, reader: jspb.BinaryReader): Resource;
}

export namespace Resource {
  export type AsObject = {
  }
}

export class TiltBuild extends jspb.Message {
  getVersion(): string;
  setVersion(value: string): void;

  getCommitSha(): string;
  setCommitSha(value: string): void;

  getDate(): string;
  setDate(value: string): void;

  getDev(): boolean;
  setDev(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): TiltBuild.AsObject;
  static toObject(includeInstance: boolean, msg: TiltBuild): TiltBuild.AsObject;
  static serializeBinaryToWriter(message: TiltBuild, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): TiltBuild;
  static deserializeBinaryFromReader(message: TiltBuild, reader: jspb.BinaryReader): TiltBuild;
}

export namespace TiltBuild {
  export type AsObject = {
    version: string,
    commitSha: string,
    date: string,
    dev: boolean,
  }
}

export class View extends jspb.Message {
  getLog(): string;
  setLog(value: string): void;

  getResourcesList(): Array<Resource>;
  setResourcesList(value: Array<Resource>): void;
  clearResourcesList(): void;
  addResources(value?: Resource, index?: number): Resource;

  getLogTimestamps(): boolean;
  setLogTimestamps(value: boolean): void;

  getFeatureFlagsMap(): jspb.Map<string, boolean>;
  clearFeatureFlagsMap(): void;

  getNeedAnalyticsNudge(): boolean;
  setNeedAnalyticsNudge(value: boolean): void;

  getRunningTiltBuild(): TiltBuild | undefined;
  setRunningTiltBuild(value?: TiltBuild): void;
  hasRunningTiltBuild(): boolean;
  clearRunningTiltBuild(): void;

  getLatestTiltBuild(): TiltBuild | undefined;
  setLatestTiltBuild(value?: TiltBuild): void;
  hasLatestTiltBuild(): boolean;
  clearLatestTiltBuild(): void;

  getTiltCloudUsername(): string;
  setTiltCloudUsername(value: string): void;

  getTiltCloudSchemehost(): string;
  setTiltCloudSchemehost(value: string): void;

  getTiltCloudTeamId(): string;
  setTiltCloudTeamId(value: string): void;

  getFatalError(): string;
  setFatalError(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): View.AsObject;
  static toObject(includeInstance: boolean, msg: View): View.AsObject;
  static serializeBinaryToWriter(message: View, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): View;
  static deserializeBinaryFromReader(message: View, reader: jspb.BinaryReader): View;
}

export namespace View {
  export type AsObject = {
    log: string,
    resourcesList: Array<Resource.AsObject>,
    logTimestamps: boolean,
    featureFlagsMap: Array<[string, boolean]>,
    needAnalyticsNudge: boolean,
    runningTiltBuild?: TiltBuild.AsObject,
    latestTiltBuild?: TiltBuild.AsObject,
    tiltCloudUsername: string,
    tiltCloudSchemehost: string,
    tiltCloudTeamId: string,
    fatalError: string,
  }
}

export class GetViewRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetViewRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetViewRequest): GetViewRequest.AsObject;
  static serializeBinaryToWriter(message: GetViewRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetViewRequest;
  static deserializeBinaryFromReader(message: GetViewRequest, reader: jspb.BinaryReader): GetViewRequest;
}

export namespace GetViewRequest {
  export type AsObject = {
  }
}

