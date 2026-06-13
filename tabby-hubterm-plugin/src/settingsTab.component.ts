import { Component } from '@angular/core'
import { NgbModal } from '@ng-bootstrap/ng-bootstrap'
import { ConfigService } from 'tabby-core'

@Component({
    template: require('./settingsTab.component.pug'),
})
export class HubTermSettingsTabComponent {
    constructor(
        public config: ConfigService,
    ) { }
}
