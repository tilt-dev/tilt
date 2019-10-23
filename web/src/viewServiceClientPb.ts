/**
 * @fileoverview gRPC-Web generated client stub for webview
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!

import * as grpcWeb from "grpc-web"

// NOTE(dmiller): why do we have to delete this?
//import * as google_api_annotations_pb from '../../google/api/annotations_pb';

import { GetViewRequest, View } from "./view_pb"

export class ViewServiceClient {
  client_: grpcWeb.AbstractClientBase
  hostname_: string
  credentials_: null | { [index: string]: string }
  options_: null | { [index: string]: string }

  constructor(
    hostname: string,
    credentials?: null | { [index: string]: string },
    options?: null | { [index: string]: string }
  ) {
    if (!options) options = {}
    if (!credentials) credentials = {}
    options["format"] = "text"

    this.client_ = new grpcWeb.GrpcWebClientBase(options)
    this.hostname_ = hostname
    this.credentials_ = credentials
    this.options_ = options
  }

  methodInfoGetView = new grpcWeb.AbstractClientBase.MethodInfo(
    View,
    (request: GetViewRequest) => {
      return request.serializeBinary()
    },
    View.deserializeBinary
  )

  getView(
    request: GetViewRequest,
    metadata: grpcWeb.Metadata | null,
    callback: (err: grpcWeb.Error, response: View) => void
  ) {
    return this.client_.rpcCall(
      this.hostname_ + "/webview.ViewService/GetView",
      request,
      metadata || {},
      this.methodInfoGetView,
      callback
    )
  }
}
