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
            tab.output$.subscribe((data: any) => {
                const str = typeof data === 'string' ? data : String(data)
                this.hubterm.sendTerminalData(tab, str)
            })
        }

        // Hook terminal input: forward to HubTerm
        if (tab.input$) {
            tab.input$.subscribe((data: any) => {
                const str = typeof data === 'string' ? data : String(data)
                this.hubterm.sendTerminalData(tab, str)
            })
        }
    }

    detach(tab: BaseTerminalTabComponent): void {
        this.hubterm.detachTab(tab)
    }
}
