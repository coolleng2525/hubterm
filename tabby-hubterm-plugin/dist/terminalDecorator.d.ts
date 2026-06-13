import { Injector } from '@angular/core';
import { TerminalDecorator } from 'tabby-terminal';
import { BaseTerminalTabComponent } from 'tabby-terminal';
export declare class HubTermDecorator extends TerminalDecorator {
    private injector;
    private hubterm;
    constructor(injector: Injector);
    attach(tab: BaseTerminalTabComponent): void;
    detach(tab: BaseTerminalTabComponent): void;
}
