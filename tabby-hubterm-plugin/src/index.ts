import { NgModule } from '@angular/core'
import { CommonModule } from '@angular/common'
import { FormsModule } from '@angular/forms'
import TabbyCoreModule, { ConfigProvider } from 'tabby-core'
import { TerminalDecorator } from 'tabby-terminal'
import { SettingsTabProvider } from 'tabby-settings'

import { HubTermConfigProvider } from './configProvider'
import { HubTermDecorator } from './terminalDecorator'
import { HubTermService } from './hubterm.service'
import { HubTermSettingsTabProvider } from './settingsTabProvider'
import { HubTermSettingsTabComponent } from './settingsTab.component'

@NgModule({
    imports: [
        CommonModule,
        FormsModule,
        TabbyCoreModule,
    ],
    providers: [
        HubTermService,
        { provide: ConfigProvider, useClass: HubTermConfigProvider, multi: true },
        { provide: SettingsTabProvider, useClass: HubTermSettingsTabProvider, multi: true },
        { provide: TerminalDecorator, useClass: HubTermDecorator, multi: true },
    ],
    entryComponents: [
        HubTermSettingsTabComponent,
    ],
    declarations: [
        HubTermSettingsTabComponent,
    ],
})
export default class HubTermModule { }
