export namespace config {
	
	export class DMXAPIConfig {
	    baseUrl: string;
	    apiKey: string;
	    currentModel: string;
	
	    static createFrom(source: any = {}) {
	        return new DMXAPIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.currentModel = source["currentModel"];
	    }
	}

}

