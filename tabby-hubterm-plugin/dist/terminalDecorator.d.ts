import { Injector } from '@angular/core';
import { TerminalDecorator, BaseTerminalTabComponent } from 'tabby-terminal';
/** @hidden */
export declare class HubTermDecorator extends TerminalDecorator {
    private injector;
    private hubterm;
    private subscriptions;
    constructor(injector: Injector);
    attach(tab: BaseTerminalTabComponent): void;
    detach(tab: BaseTerminalTabComponent): void;
}
