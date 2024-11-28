// Generated from ArchDSL.g4 by ANTLR 4.13.2
// jshint ignore: start
import antlr4 from 'antlr4';
import ArchDSLListener from './ArchDSLListener.js';
const serializedATN = [4,1,29,127,2,0,7,0,2,1,7,1,2,2,7,2,2,3,7,3,2,4,7,
4,2,5,7,5,2,6,7,6,2,7,7,7,2,8,7,8,2,9,7,9,2,10,7,10,2,11,7,11,2,12,7,12,
2,13,7,13,1,0,5,0,30,8,0,10,0,12,0,33,9,0,1,0,5,0,36,8,0,10,0,12,0,39,9,
0,1,1,1,1,1,1,1,1,5,1,45,8,1,10,1,12,1,48,9,1,1,1,1,1,1,2,1,2,1,2,1,2,1,
2,1,2,1,2,1,2,1,2,5,2,61,8,2,10,2,12,2,64,9,2,1,2,1,2,1,3,1,3,1,3,1,4,1,
4,1,4,3,4,74,8,4,1,5,1,5,1,5,3,5,79,8,5,1,5,3,5,82,8,5,1,6,1,6,1,6,1,7,1,
7,1,7,1,7,1,7,1,7,1,8,1,8,1,8,1,9,1,9,1,9,1,10,1,10,1,11,1,11,1,11,1,11,
1,11,3,11,106,8,11,1,11,1,11,1,11,3,11,111,8,11,1,12,1,12,1,12,1,12,1,12,
1,12,1,13,1,13,1,13,5,13,122,8,13,10,13,12,13,125,9,13,1,13,0,0,14,0,2,4,
6,8,10,12,14,16,18,20,22,24,26,0,2,1,0,10,11,1,0,16,18,126,0,31,1,0,0,0,
2,40,1,0,0,0,4,51,1,0,0,0,6,67,1,0,0,0,8,70,1,0,0,0,10,75,1,0,0,0,12,83,
1,0,0,0,14,86,1,0,0,0,16,92,1,0,0,0,18,95,1,0,0,0,20,98,1,0,0,0,22,100,1,
0,0,0,24,112,1,0,0,0,26,118,1,0,0,0,28,30,3,2,1,0,29,28,1,0,0,0,30,33,1,
0,0,0,31,29,1,0,0,0,31,32,1,0,0,0,32,37,1,0,0,0,33,31,1,0,0,0,34,36,3,22,
11,0,35,34,1,0,0,0,36,39,1,0,0,0,37,35,1,0,0,0,37,38,1,0,0,0,38,1,1,0,0,
0,39,37,1,0,0,0,40,41,5,1,0,0,41,42,5,26,0,0,42,46,5,2,0,0,43,45,3,4,2,0,
44,43,1,0,0,0,45,48,1,0,0,0,46,44,1,0,0,0,46,47,1,0,0,0,47,49,1,0,0,0,48,
46,1,0,0,0,49,50,5,3,0,0,50,3,1,0,0,0,51,52,5,4,0,0,52,53,5,5,0,0,53,54,
5,26,0,0,54,62,5,2,0,0,55,61,3,6,3,0,56,61,3,8,4,0,57,61,3,10,5,0,58,61,
3,12,6,0,59,61,3,14,7,0,60,55,1,0,0,0,60,56,1,0,0,0,60,57,1,0,0,0,60,58,
1,0,0,0,60,59,1,0,0,0,61,64,1,0,0,0,62,60,1,0,0,0,62,63,1,0,0,0,63,65,1,
0,0,0,64,62,1,0,0,0,65,66,5,3,0,0,66,5,1,0,0,0,67,68,5,6,0,0,68,69,5,26,
0,0,69,7,1,0,0,0,70,71,5,7,0,0,71,73,5,26,0,0,72,74,3,16,8,0,73,72,1,0,0,
0,73,74,1,0,0,0,74,9,1,0,0,0,75,76,5,8,0,0,76,78,5,26,0,0,77,79,3,16,8,0,
78,77,1,0,0,0,78,79,1,0,0,0,79,81,1,0,0,0,80,82,3,18,9,0,81,80,1,0,0,0,81,
82,1,0,0,0,82,11,1,0,0,0,83,84,5,9,0,0,84,85,5,26,0,0,85,13,1,0,0,0,86,87,
7,0,0,0,87,88,5,12,0,0,88,89,5,26,0,0,89,90,5,13,0,0,90,91,3,20,10,0,91,
15,1,0,0,0,92,93,5,14,0,0,93,94,5,24,0,0,94,17,1,0,0,0,95,96,5,15,0,0,96,
97,5,25,0,0,97,19,1,0,0,0,98,99,7,1,0,0,99,21,1,0,0,0,100,101,5,26,0,0,101,
102,5,19,0,0,102,103,5,26,0,0,103,105,5,20,0,0,104,106,3,26,13,0,105,104,
1,0,0,0,105,106,1,0,0,0,106,107,1,0,0,0,107,110,5,21,0,0,108,109,5,22,0,
0,109,111,3,24,12,0,110,108,1,0,0,0,110,111,1,0,0,0,111,23,1,0,0,0,112,113,
5,26,0,0,113,114,5,19,0,0,114,115,5,26,0,0,115,116,5,20,0,0,116,117,5,21,
0,0,117,25,1,0,0,0,118,123,5,26,0,0,119,120,5,23,0,0,120,122,5,26,0,0,121,
119,1,0,0,0,122,125,1,0,0,0,123,121,1,0,0,0,123,124,1,0,0,0,124,27,1,0,0,
0,125,123,1,0,0,0,11,31,37,46,60,62,73,78,81,105,110,123];


const atn = new antlr4.atn.ATNDeserializer().deserialize(serializedATN);

const decisionsToDFA = atn.decisionToState.map( (ds, index) => new antlr4.dfa.DFA(ds, index) );

const sharedContextCache = new antlr4.atn.PredictionContextCache();

export default class ArchDSLParser extends antlr4.Parser {

    static grammarFileName = "ArchDSL.g4";
    static literalNames = [ null, "'system'", "'{'", "'}'", "'bounded'", 
                            "'context'", "'aggregate'", "'component'", "'service'", 
                            "'event'", "'upstream'", "'downstream'", "'to'", 
                            "'as'", "'using'", "'on'", "'acl'", "'ohs'", 
                            "'conformist'", "'.'", "'('", "')'", "'->'", 
                            "','" ];
    static symbolicNames = [ null, null, null, null, null, null, null, null, 
                             null, null, null, null, null, null, null, null, 
                             null, null, null, null, null, null, null, null, 
                             "TECH", "PLATFORM", "IDENT", "WS", "LINE_COMMENT", 
                             "BLOCK_COMMENT" ];
    static ruleNames = [ "architecture", "system", "context", "aggregate", 
                         "component", "service", "event", "relation", "tech", 
                         "platform", "pattern", "flow", "target", "args" ];

    constructor(input) {
        super(input);
        this._interp = new antlr4.atn.ParserATNSimulator(this, atn, decisionsToDFA, sharedContextCache);
        this.ruleNames = ArchDSLParser.ruleNames;
        this.literalNames = ArchDSLParser.literalNames;
        this.symbolicNames = ArchDSLParser.symbolicNames;
    }



	architecture() {
	    let localctx = new ArchitectureContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 0, ArchDSLParser.RULE_architecture);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 31;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        while(_la===1) {
	            this.state = 28;
	            this.system();
	            this.state = 33;
	            this._errHandler.sync(this);
	            _la = this._input.LA(1);
	        }
	        this.state = 37;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        while(_la===26) {
	            this.state = 34;
	            this.flow();
	            this.state = 39;
	            this._errHandler.sync(this);
	            _la = this._input.LA(1);
	        }
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	system() {
	    let localctx = new SystemContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 2, ArchDSLParser.RULE_system);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 40;
	        this.match(ArchDSLParser.T__0);
	        this.state = 41;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 42;
	        this.match(ArchDSLParser.T__1);
	        this.state = 46;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        while(_la===4) {
	            this.state = 43;
	            this.context();
	            this.state = 48;
	            this._errHandler.sync(this);
	            _la = this._input.LA(1);
	        }
	        this.state = 49;
	        this.match(ArchDSLParser.T__2);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	context() {
	    let localctx = new ContextContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 4, ArchDSLParser.RULE_context);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 51;
	        this.match(ArchDSLParser.T__3);
	        this.state = 52;
	        this.match(ArchDSLParser.T__4);
	        this.state = 53;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 54;
	        this.match(ArchDSLParser.T__1);
	        this.state = 62;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        while((((_la) & ~0x1f) === 0 && ((1 << _la) & 4032) !== 0)) {
	            this.state = 60;
	            this._errHandler.sync(this);
	            switch(this._input.LA(1)) {
	            case 6:
	                this.state = 55;
	                this.aggregate();
	                break;
	            case 7:
	                this.state = 56;
	                this.component();
	                break;
	            case 8:
	                this.state = 57;
	                this.service();
	                break;
	            case 9:
	                this.state = 58;
	                this.event();
	                break;
	            case 10:
	            case 11:
	                this.state = 59;
	                this.relation();
	                break;
	            default:
	                throw new antlr4.error.NoViableAltException(this);
	            }
	            this.state = 64;
	            this._errHandler.sync(this);
	            _la = this._input.LA(1);
	        }
	        this.state = 65;
	        this.match(ArchDSLParser.T__2);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	aggregate() {
	    let localctx = new AggregateContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 6, ArchDSLParser.RULE_aggregate);
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 67;
	        this.match(ArchDSLParser.T__5);
	        this.state = 68;
	        this.match(ArchDSLParser.IDENT);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	component() {
	    let localctx = new ComponentContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 8, ArchDSLParser.RULE_component);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 70;
	        this.match(ArchDSLParser.T__6);
	        this.state = 71;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 73;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        if(_la===14) {
	            this.state = 72;
	            this.tech();
	        }

	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	service() {
	    let localctx = new ServiceContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 10, ArchDSLParser.RULE_service);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 75;
	        this.match(ArchDSLParser.T__7);
	        this.state = 76;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 78;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        if(_la===14) {
	            this.state = 77;
	            this.tech();
	        }

	        this.state = 81;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        if(_la===15) {
	            this.state = 80;
	            this.platform();
	        }

	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	event() {
	    let localctx = new EventContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 12, ArchDSLParser.RULE_event);
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 83;
	        this.match(ArchDSLParser.T__8);
	        this.state = 84;
	        this.match(ArchDSLParser.IDENT);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	relation() {
	    let localctx = new RelationContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 14, ArchDSLParser.RULE_relation);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 86;
	        _la = this._input.LA(1);
	        if(!(_la===10 || _la===11)) {
	        this._errHandler.recoverInline(this);
	        }
	        else {
	        	this._errHandler.reportMatch(this);
	            this.consume();
	        }
	        this.state = 87;
	        this.match(ArchDSLParser.T__11);
	        this.state = 88;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 89;
	        this.match(ArchDSLParser.T__12);
	        this.state = 90;
	        this.pattern();
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	tech() {
	    let localctx = new TechContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 16, ArchDSLParser.RULE_tech);
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 92;
	        this.match(ArchDSLParser.T__13);
	        this.state = 93;
	        this.match(ArchDSLParser.TECH);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	platform() {
	    let localctx = new PlatformContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 18, ArchDSLParser.RULE_platform);
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 95;
	        this.match(ArchDSLParser.T__14);
	        this.state = 96;
	        this.match(ArchDSLParser.PLATFORM);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	pattern() {
	    let localctx = new PatternContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 20, ArchDSLParser.RULE_pattern);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 98;
	        _la = this._input.LA(1);
	        if(!((((_la) & ~0x1f) === 0 && ((1 << _la) & 458752) !== 0))) {
	        this._errHandler.recoverInline(this);
	        }
	        else {
	        	this._errHandler.reportMatch(this);
	            this.consume();
	        }
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	flow() {
	    let localctx = new FlowContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 22, ArchDSLParser.RULE_flow);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 100;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 101;
	        this.match(ArchDSLParser.T__18);
	        this.state = 102;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 103;
	        this.match(ArchDSLParser.T__19);
	        this.state = 105;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        if(_la===26) {
	            this.state = 104;
	            this.args();
	        }

	        this.state = 107;
	        this.match(ArchDSLParser.T__20);
	        this.state = 110;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        if(_la===22) {
	            this.state = 108;
	            this.match(ArchDSLParser.T__21);
	            this.state = 109;
	            this.target();
	        }

	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	target() {
	    let localctx = new TargetContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 24, ArchDSLParser.RULE_target);
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 112;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 113;
	        this.match(ArchDSLParser.T__18);
	        this.state = 114;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 115;
	        this.match(ArchDSLParser.T__19);
	        this.state = 116;
	        this.match(ArchDSLParser.T__20);
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}



	args() {
	    let localctx = new ArgsContext(this, this._ctx, this.state);
	    this.enterRule(localctx, 26, ArchDSLParser.RULE_args);
	    var _la = 0;
	    try {
	        this.enterOuterAlt(localctx, 1);
	        this.state = 118;
	        this.match(ArchDSLParser.IDENT);
	        this.state = 123;
	        this._errHandler.sync(this);
	        _la = this._input.LA(1);
	        while(_la===23) {
	            this.state = 119;
	            this.match(ArchDSLParser.T__22);
	            this.state = 120;
	            this.match(ArchDSLParser.IDENT);
	            this.state = 125;
	            this._errHandler.sync(this);
	            _la = this._input.LA(1);
	        }
	    } catch (re) {
	    	if(re instanceof antlr4.error.RecognitionException) {
		        localctx.exception = re;
		        this._errHandler.reportError(this, re);
		        this._errHandler.recover(this, re);
		    } else {
		    	throw re;
		    }
	    } finally {
	        this.exitRule();
	    }
	    return localctx;
	}


}

ArchDSLParser.EOF = antlr4.Token.EOF;
ArchDSLParser.T__0 = 1;
ArchDSLParser.T__1 = 2;
ArchDSLParser.T__2 = 3;
ArchDSLParser.T__3 = 4;
ArchDSLParser.T__4 = 5;
ArchDSLParser.T__5 = 6;
ArchDSLParser.T__6 = 7;
ArchDSLParser.T__7 = 8;
ArchDSLParser.T__8 = 9;
ArchDSLParser.T__9 = 10;
ArchDSLParser.T__10 = 11;
ArchDSLParser.T__11 = 12;
ArchDSLParser.T__12 = 13;
ArchDSLParser.T__13 = 14;
ArchDSLParser.T__14 = 15;
ArchDSLParser.T__15 = 16;
ArchDSLParser.T__16 = 17;
ArchDSLParser.T__17 = 18;
ArchDSLParser.T__18 = 19;
ArchDSLParser.T__19 = 20;
ArchDSLParser.T__20 = 21;
ArchDSLParser.T__21 = 22;
ArchDSLParser.T__22 = 23;
ArchDSLParser.TECH = 24;
ArchDSLParser.PLATFORM = 25;
ArchDSLParser.IDENT = 26;
ArchDSLParser.WS = 27;
ArchDSLParser.LINE_COMMENT = 28;
ArchDSLParser.BLOCK_COMMENT = 29;

ArchDSLParser.RULE_architecture = 0;
ArchDSLParser.RULE_system = 1;
ArchDSLParser.RULE_context = 2;
ArchDSLParser.RULE_aggregate = 3;
ArchDSLParser.RULE_component = 4;
ArchDSLParser.RULE_service = 5;
ArchDSLParser.RULE_event = 6;
ArchDSLParser.RULE_relation = 7;
ArchDSLParser.RULE_tech = 8;
ArchDSLParser.RULE_platform = 9;
ArchDSLParser.RULE_pattern = 10;
ArchDSLParser.RULE_flow = 11;
ArchDSLParser.RULE_target = 12;
ArchDSLParser.RULE_args = 13;

class ArchitectureContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_architecture;
    }

	system = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(SystemContext);
	    } else {
	        return this.getTypedRuleContext(SystemContext,i);
	    }
	};

	flow = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(FlowContext);
	    } else {
	        return this.getTypedRuleContext(FlowContext,i);
	    }
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterArchitecture(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitArchitecture(this);
		}
	}


}



class SystemContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_system;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	context = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(ContextContext);
	    } else {
	        return this.getTypedRuleContext(ContextContext,i);
	    }
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterSystem(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitSystem(this);
		}
	}


}



class ContextContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_context;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	aggregate = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(AggregateContext);
	    } else {
	        return this.getTypedRuleContext(AggregateContext,i);
	    }
	};

	component = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(ComponentContext);
	    } else {
	        return this.getTypedRuleContext(ComponentContext,i);
	    }
	};

	service = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(ServiceContext);
	    } else {
	        return this.getTypedRuleContext(ServiceContext,i);
	    }
	};

	event = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(EventContext);
	    } else {
	        return this.getTypedRuleContext(EventContext,i);
	    }
	};

	relation = function(i) {
	    if(i===undefined) {
	        i = null;
	    }
	    if(i===null) {
	        return this.getTypedRuleContexts(RelationContext);
	    } else {
	        return this.getTypedRuleContext(RelationContext,i);
	    }
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterContext(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitContext(this);
		}
	}


}



class AggregateContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_aggregate;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterAggregate(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitAggregate(this);
		}
	}


}



class ComponentContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_component;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	tech() {
	    return this.getTypedRuleContext(TechContext,0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterComponent(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitComponent(this);
		}
	}


}



class ServiceContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_service;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	tech() {
	    return this.getTypedRuleContext(TechContext,0);
	};

	platform() {
	    return this.getTypedRuleContext(PlatformContext,0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterService(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitService(this);
		}
	}


}



class EventContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_event;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterEvent(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitEvent(this);
		}
	}


}



class RelationContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_relation;
    }

	IDENT() {
	    return this.getToken(ArchDSLParser.IDENT, 0);
	};

	pattern() {
	    return this.getTypedRuleContext(PatternContext,0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterRelation(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitRelation(this);
		}
	}


}



class TechContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_tech;
    }

	TECH() {
	    return this.getToken(ArchDSLParser.TECH, 0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterTech(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitTech(this);
		}
	}


}



class PlatformContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_platform;
    }

	PLATFORM() {
	    return this.getToken(ArchDSLParser.PLATFORM, 0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterPlatform(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitPlatform(this);
		}
	}


}



class PatternContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_pattern;
    }


	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterPattern(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitPattern(this);
		}
	}


}



class FlowContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_flow;
    }

	IDENT = function(i) {
		if(i===undefined) {
			i = null;
		}
	    if(i===null) {
	        return this.getTokens(ArchDSLParser.IDENT);
	    } else {
	        return this.getToken(ArchDSLParser.IDENT, i);
	    }
	};


	args() {
	    return this.getTypedRuleContext(ArgsContext,0);
	};

	target() {
	    return this.getTypedRuleContext(TargetContext,0);
	};

	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterFlow(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitFlow(this);
		}
	}


}



class TargetContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_target;
    }

	IDENT = function(i) {
		if(i===undefined) {
			i = null;
		}
	    if(i===null) {
	        return this.getTokens(ArchDSLParser.IDENT);
	    } else {
	        return this.getToken(ArchDSLParser.IDENT, i);
	    }
	};


	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterTarget(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitTarget(this);
		}
	}


}



class ArgsContext extends antlr4.ParserRuleContext {

    constructor(parser, parent, invokingState) {
        if(parent===undefined) {
            parent = null;
        }
        if(invokingState===undefined || invokingState===null) {
            invokingState = -1;
        }
        super(parent, invokingState);
        this.parser = parser;
        this.ruleIndex = ArchDSLParser.RULE_args;
    }

	IDENT = function(i) {
		if(i===undefined) {
			i = null;
		}
	    if(i===null) {
	        return this.getTokens(ArchDSLParser.IDENT);
	    } else {
	        return this.getToken(ArchDSLParser.IDENT, i);
	    }
	};


	enterRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.enterArgs(this);
		}
	}

	exitRule(listener) {
	    if(listener instanceof ArchDSLListener ) {
	        listener.exitArgs(this);
		}
	}


}




ArchDSLParser.ArchitectureContext = ArchitectureContext; 
ArchDSLParser.SystemContext = SystemContext; 
ArchDSLParser.ContextContext = ContextContext; 
ArchDSLParser.AggregateContext = AggregateContext; 
ArchDSLParser.ComponentContext = ComponentContext; 
ArchDSLParser.ServiceContext = ServiceContext; 
ArchDSLParser.EventContext = EventContext; 
ArchDSLParser.RelationContext = RelationContext; 
ArchDSLParser.TechContext = TechContext; 
ArchDSLParser.PlatformContext = PlatformContext; 
ArchDSLParser.PatternContext = PatternContext; 
ArchDSLParser.FlowContext = FlowContext; 
ArchDSLParser.TargetContext = TargetContext; 
ArchDSLParser.ArgsContext = ArgsContext; 
