import { SettingsTabProvider } from 'tabby-settings';
import { HubTermSettingsTabComponent } from './settingsTab.component';
/** @hidden */
export declare class HubTermSettingsTabProvider extends SettingsTabProvider {
    constructor();
    getTitle(): string;
    getComponent(): typeof HubTermSettingsTabComponent;
}
