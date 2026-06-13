import { Injectable, Injector } from '@angular/core'
import { TerminalDecorator } from 'tabby-terminal'
import { BaseTerminalTabComponent } from 'tabby-terminal'
import { HubTermService } from './hubterm.service'

@Injectable()
export class HubTermDecorator extends TerminalDecorator {
    private hubterm: HubTermService

    constructor(
        private injector: Injector,
    ) {
        super()
        this.hubterm = this.injector.get(HubTermService)
    }

    attach(tab: BaseTerminalTabComponent): void {
        this.hubterm.attachTab(tab)

        // Hook terminal output: forward to HubTerm
        if (tab.output$) {
            tab.output$.subscribe(data => {
                this.hubterm.sendTerminalData(tab, data)
            })
        }

        // Hook terminal input: forward to HubTerm
        if (tab.input$) {
            tab.input$.subscribe(data => {
                this.hubterm.sendTerminalData(tab, data)
            })
        }
    }

    detach(tab: BaseTerminalTabComponent): void {
        this.hubterm.detachTab(tab)
    }
}
