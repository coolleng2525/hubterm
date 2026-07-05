import { ConfigProvider } from 'tabby-core';
export interface HubTermConfig {
    enabled: boolean;
    centerUrl: string;
    nodeName: string;
    domain: string;
    token: string;
    reportInterval: number;
}
/** @hidden */
export declare class HubTermConfigProvider extends ConfigProvider {
    defaults: any;
    platformDefaults: any;
}
