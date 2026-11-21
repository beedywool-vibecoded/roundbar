const require_runtime = require("../_virtual/_rolldown/runtime.js");
const require_scope = require("./scope.js");
const require_ts_types = require("./ts-utils/ts-types.js");
const require_ts_ast = require("./ts-utils/ts-ast.js");
const require_index = require("./ts-utils/index.js");
const require_html_elements = require("./html-elements.json.js");
const require_svg_elements = require("./svg-elements.json.js");
const require_math_elements = require("./math-elements.json.js");
const require_void_elements = require("./void-elements.json.js");
const require_vue2_builtin_components = require("./vue2-builtin-components.js");
const require_vue3_builtin_components = require("./vue3-builtin-components.js");
const require_vue_builtin_elements = require("./vue-builtin-elements.js");
let eslint = require("eslint");
let eslint_use_at_your_own_risk = require("eslint/use-at-your-own-risk");
let node_module = require("node:module");
node_module = require_runtime.__toESM(node_module);
let node_path = require("node:path");
node_path = require_runtime.__toESM(node_path);
let vue_eslint_parser = require("vue-eslint-parser");
vue_eslint_parser = require_runtime.__toESM(vue_eslint_parser);
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/utils/index.ts
const HTML_ELEMENT_NAMES = new Set(require_html_elements.default);
const SVG_ELEMENT_NAMES = new Set(require_svg_elements.default);
const MATH_ELEMENT_NAMES = new Set(require_math_elements.default);
const VOID_ELEMENT_NAMES = new Set(require_void_elements.default);
const VUE2_BUILTIN_COMPONENT_NAMES = new Set(require_vue2_builtin_components.default);
const VUE3_BUILTIN_COMPONENT_NAMES = new Set(require_vue3_builtin_components.default);
const VUE_BUILTIN_ELEMENT_NAMES = new Set(require_vue_builtin_elements.default);
const BUILTIN_DIRECTIVE_NAMES = /* @__PURE__ */ new Set([
	"bind",
	"on",
	"text",
	"html",
	"show",
	"if",
	"else",
	"else-if",
	"for",
	"model",
	"slot",
	"pre",
	"cloak",
	"once",
	"memo",
	"is"
]);
const { traverseNodes, getFallbackKeys, NS } = vue_eslint_parser.default.AST;
const componentComments = /* @__PURE__ */ new WeakMap();
let coreRuleMap = null;
const stylisticRuleMap = /* @__PURE__ */ new Map();
/**
* Get the core rule implementation from the rule name
*/
function getCoreRule(name) {
	try {
		coreRuleMap ||= new eslint.Linter().getRules();
		return coreRuleMap.get(name) || null;
	} catch {}
	return eslint_use_at_your_own_risk.builtinRules.get(name) || null;
}
/**
* Get ESLint Stylistic rule implementation from the rule name
*/
function getStylisticRule(name, preferModule) {
	if (!preferModule) {
		const cached = stylisticRuleMap.get(name);
		if (cached) return cached;
	}
	const stylisticPluginNames = [
		"@stylistic/eslint-plugin",
		"@stylistic/eslint-plugin-ts",
		"@stylistic/eslint-plugin-js"
	];
	if (preferModule) stylisticPluginNames.unshift(preferModule);
	for (const stylisticPluginName of stylisticPluginNames) try {
		const rule = node_module.default.createRequire(`${process.cwd()}/__placeholder__.js`)(stylisticPluginName)?.rules?.[name];
		if (!preferModule) stylisticRuleMap.set(name, rule);
		return rule;
	} catch {}
	return null;
}
function newProxy(target, ...propsArray) {
	return new Proxy({}, {
		get(_object, key) {
			for (const props of propsArray) if (key in props) return props[key];
			return target[key];
		},
		has(_object, key) {
			return key in target;
		},
		ownKeys(_object) {
			return Reflect.ownKeys(target);
		},
		getPrototypeOf(_object) {
			return Reflect.getPrototypeOf(target);
		}
	});
}
/**
* Wrap the rule context object to override methods which access to tokens (such as getTokenAfter).
*/
function wrapContextToOverrideTokenMethods(context, tokenStore, options) {
	const eslintSourceCode = context.sourceCode;
	const rootNode = options.applyDocument ? eslintSourceCode.parserServices.getDocumentFragment && eslintSourceCode.parserServices.getDocumentFragment() : eslintSourceCode.ast.templateBody;
	let tokensAndComments = null;
	function getTokensAndComments() {
		if (tokensAndComments) return tokensAndComments;
		tokensAndComments = rootNode ? tokenStore.getTokens(rootNode, { includeComments: true }) : [];
		return tokensAndComments;
	}
	function getNodeByRangeIndex(index) {
		if (!rootNode) return eslintSourceCode.ast;
		let result = eslintSourceCode.ast;
		const skipNodes = [];
		let shouldBreak = false;
		traverseNodes(rootNode, {
			enterNode(node, parent) {
				if (shouldBreak) return;
				if (skipNodes[0] === parent) {
					skipNodes.unshift(node);
					return;
				}
				if (node.range[0] <= index && index < node.range[1]) result = node;
				else skipNodes.unshift(node);
			},
			leaveNode(node) {
				if (shouldBreak) return;
				if (result === node) shouldBreak = true;
				else if (skipNodes[0] === node) skipNodes.shift();
			}
		});
		return result;
	}
	const sourceCode = newProxy(eslintSourceCode, {
		get tokensAndComments() {
			return getTokensAndComments();
		},
		getNodeByRangeIndex,
		getDeclaredVariables
	}, tokenStore);
	const containerScopes = /* @__PURE__ */ new WeakMap();
	function getContainerScope(node) {
		const exprContainer = getVExpressionContainer(node);
		if (!exprContainer) return null;
		const cache = containerScopes.get(exprContainer);
		if (cache) return cache;
		const programNode = eslintSourceCode.ast;
		const parserOptions = context.languageOptions?.parserOptions ?? context.parserOptions ?? {};
		const ecmaFeatures = parserOptions.ecmaFeatures || {};
		const ecmaVersion = context.languageOptions?.ecmaVersion ?? parserOptions.ecmaVersion ?? 2020;
		const sourceType = programNode.sourceType;
		try {
			const eslintScope = node_module.default.createRequire(require.resolve("eslint"))("eslint-scope");
			const scopeProgram = newProxy(programNode, { body: [newProxy(exprContainer, { type: "ExpressionStatement" })] });
			const scope = eslintScope.analyze(scopeProgram, {
				ignoreEval: true,
				nodejsScope: false,
				impliedStrict: ecmaFeatures.impliedStrict,
				ecmaVersion,
				sourceType,
				fallback: getFallbackKeys
			});
			containerScopes.set(exprContainer, scope);
			return scope;
		} catch {}
		return null;
	}
	return newProxy(context, {
		getSourceCode() {
			return sourceCode;
		},
		get sourceCode() {
			return sourceCode;
		},
		getDeclaredVariables
	});
	function getDeclaredVariables(node) {
		return getContainerScope(node)?.getDeclaredVariables(node) ?? [];
	}
}
/**
* Wrap the rule context object to override report method to skip the dynamic argument.
*/
function wrapContextToOverrideReportMethodToSkipDynamicArgument(context) {
	const sourceCode = context.sourceCode;
	const templateBody = sourceCode.ast.templateBody;
	if (!templateBody) return context;
	const directiveKeyRanges = [];
	traverseNodes(templateBody, {
		enterNode(node, parent) {
			if (parent && parent.type === "VDirectiveKey" && node.type === "VExpressionContainer") directiveKeyRanges.push(node.range);
		},
		leaveNode() {}
	});
	return newProxy(context, { report(descriptor, ...args) {
		let range = null;
		if (descriptor.loc) {
			const startLoc = descriptor.loc.start || descriptor.loc;
			const endLoc = descriptor.loc.end || startLoc;
			range = [sourceCode.getIndexFromLoc(startLoc), sourceCode.getIndexFromLoc(endLoc)];
		} else if (descriptor.node) range = descriptor.node.range;
		if (range) {
			for (const directiveKeyRange of directiveKeyRanges) if (range[0] < directiveKeyRange[1] && directiveKeyRange[0] < range[1]) return;
		}
		context.report(descriptor, ...args);
	} });
}
/**
* Wrap a given core rule to apply it to Vue.js template.
*/
function wrapCoreRule(coreRuleName, options) {
	const coreRule = getCoreRule(coreRuleName);
	if (!coreRule) return {
		meta: {
			type: "problem",
			docs: { url: `https://eslint.vuejs.org/rules/${coreRuleName}.html` }
		},
		create(context) {
			return defineTemplateBodyVisitor(context, { "VElement[name='template'][parent.type='VDocumentFragment']"(node) {
				context.report({
					node,
					message: `Failed to extend ESLint core rule "${coreRuleName}". You may be able to use this rule by upgrading the version of ESLint. If you cannot upgrade it, turn off this rule.`
				});
			} });
		}
	};
	const rule = wrapRuleModule(coreRule, coreRuleName, options);
	const meta = {
		...rule.meta,
		docs: {
			...rule.meta.docs,
			extensionSource: {
				url: coreRule.meta.docs.url,
				name: "ESLint core"
			}
		}
	};
	return {
		...rule,
		meta
	};
}
/**
* Wrap a core rule or ESLint Stylistic rule to apply it to Vue.js template.
*/
function wrapStylisticOrCoreRule(ruleNames, options) {
	const stylisticRuleName = typeof ruleNames === "string" ? ruleNames : ruleNames.stylistic;
	const coreRuleName = typeof ruleNames === "string" ? ruleNames : ruleNames.core;
	const vueRuleName = typeof ruleNames === "string" ? ruleNames : ruleNames.vue;
	const stylisticRule = getStylisticRule(stylisticRuleName);
	const baseRule = stylisticRule || getCoreRule(coreRuleName);
	if (!baseRule) return {
		meta: {
			type: "problem",
			docs: { url: `https://eslint.vuejs.org/rules/${vueRuleName}.html` }
		},
		create(context) {
			return defineTemplateBodyVisitor(context, { "VElement[name='template'][parent.type='VDocumentFragment']"(node) {
				context.report({
					node,
					message: `Failed to extend ESLint Stylistic rule "${stylisticRule}". You may be able to use this rule by installing ESLint Stylistic plugin (https://eslint.style/). If you cannot install it, turn off this rule.`
				});
			} });
		}
	};
	const rule = wrapRuleModule(baseRule, vueRuleName, options);
	const jsRule = getStylisticRule(stylisticRuleName, "@stylistic/eslint-plugin-js");
	const description = stylisticRule ? `${jsRule?.meta.docs.description} in \`<template>\`` : rule.meta.docs.description;
	const meta = {
		...rule.meta,
		docs: {
			...rule.meta.docs,
			description,
			extensionSource: {
				url: baseRule.meta.docs.url,
				name: stylisticRule ? "ESLint Stylistic" : "ESLint core"
			}
		},
		deprecated: void 0,
		replacedBy: void 0
	};
	return {
		...rule,
		meta
	};
}
/**
* Wrap a given rule to apply it to Vue.js template.
*/
function wrapRuleModule(baseRule, ruleName, options) {
	let description = baseRule.meta.docs.description;
	if (description) description += " in `<template>`";
	const { categories, skipDynamicArguments, skipDynamicArgumentsReport, skipBaseHandlers, applyDocument, preprocess, create } = options || {};
	return {
		create(context) {
			const sourceCode = context.sourceCode;
			const tokenStore = sourceCode.parserServices.getTemplateBodyTokenStore && sourceCode.parserServices.getTemplateBodyTokenStore();
			if (tokenStore) context = wrapContextToOverrideTokenMethods(context, tokenStore, { applyDocument });
			if (skipDynamicArgumentsReport) context = wrapContextToOverrideReportMethodToSkipDynamicArgument(context);
			const handlers = {};
			if (preprocess) preprocess(context, {
				wrapContextToOverrideProperties(override) {
					context = newProxy(context, override);
					return context;
				},
				defineVisitor(visitor) {
					compositingVisitors(handlers, visitor);
				}
			});
			const baseHandlers = baseRule.create(context);
			if (!skipBaseHandlers) compositingVisitors(handlers, baseHandlers);
			if (handlers.Program) {
				handlers[applyDocument ? "VDocumentFragment" : "VElement[parent.type!='VElement']"] = handlers.Program;
				delete handlers.Program;
			}
			if (handlers["Program:exit"]) {
				handlers[applyDocument ? "VDocumentFragment:exit" : "VElement[parent.type!='VElement']:exit"] = handlers["Program:exit"];
				delete handlers["Program:exit"];
			}
			if (skipDynamicArguments) {
				let isWithinDynamicArguments = false;
				for (const name of Object.keys(handlers)) {
					const original = handlers[name];
					handlers[name] = (...args) => {
						if (isWithinDynamicArguments) return;
						original(...args);
					};
				}
				handlers["VDirectiveKey > VExpressionContainer"] = () => {
					isWithinDynamicArguments = true;
				};
				handlers["VDirectiveKey > VExpressionContainer:exit"] = () => {
					isWithinDynamicArguments = false;
				};
			}
			if (create) compositingVisitors(handlers, create(context, { baseHandlers }));
			return applyDocument ? defineDocumentVisitor(context, handlers) : defineTemplateBodyVisitor(context, handlers);
		},
		meta: Object.assign({}, baseRule.meta, { docs: Object.assign({}, baseRule.meta.docs, {
			description,
			category: null,
			categories,
			url: `https://eslint.vuejs.org/rules/${ruleName}.html`
		}) })
	};
}
const utils = {
	/**
	* Register the given visitor to parser services.
	* If the parser service of `vue-eslint-parser` was not found,
	* this generates a warning.
	*/
	defineTemplateBodyVisitor,
	/**
	* Register the given visitor to parser services.
	* If the parser service of `vue-eslint-parser` was not found,
	* this generates a warning.
	*/
	defineDocumentVisitor,
	/**
	* Wrap a given core rule to apply it to Vue.js template.
	*/
	wrapCoreRule,
	wrapStylisticOrCoreRule,
	getCoreRule,
	/**
	* Checks whether the given value is defined.
	*/
	isDef,
	/**
	* Flattens arrays, objects and iterable objects.
	*/
	flatten,
	/**
	* Get the previous sibling element of the given element.
	*/
	prevSibling(node) {
		let prevElement = null;
		const siblings = node.parent && node.parent.children || [];
		for (const siblingNode of siblings) {
			if (siblingNode === node) return prevElement;
			if (siblingNode.type === "VElement") prevElement = siblingNode;
		}
		return null;
	},
	/**
	* Check whether the given directive attribute has their empty value (`=""`).
	*/
	isEmptyValueDirective(node, context) {
		if (node.value == null || node.value.expression != null) return false;
		let valueText = context.sourceCode.getText(node.value);
		if ((valueText[0] === "\"" || valueText[0] === "'") && valueText[0] === valueText.at(-1)) valueText = valueText.slice(1, -1);
		return !valueText;
	},
	/**
	* Check whether the given directive attribute has their empty expression value (e.g. `=" "`, `="/* &ast;/"`).
	*/
	isEmptyExpressionValueDirective(node, context) {
		if (node.value == null || node.value.expression != null) return false;
		const sourceCode = context.sourceCode;
		const valueNode = node.value;
		const tokenStore = sourceCode.parserServices.getTemplateBodyTokenStore();
		let quote1 = null;
		let quote2 = null;
		for (const token of tokenStore.getTokens(node)) {
			if (token.range[1] <= valueNode.range[0]) continue;
			if (valueNode.range[1] <= token.range[0]) return true;
			if (!quote1 && token.type === "Punctuator" && (token.value === "\"" || token.value === "'")) {
				quote1 = token;
				continue;
			}
			if (!quote2 && quote1 && token.type === "Punctuator" && token.value === quote1.value) {
				quote2 = token;
				continue;
			}
			return false;
		}
		return true;
	},
	/**
	* Get the attribute which has the given name.
	*/
	getAttribute,
	/**
	* Check whether the given start tag has specific directive.
	*/
	hasAttribute,
	/**
	* Get the directive list which has the given name.
	*/
	getDirectives,
	/**
	* Get the directive which has the given name.
	*/
	getDirective,
	/**
	* Check whether the given start tag has specific directive.
	*/
	hasDirective,
	isVBindSameNameShorthand,
	/**
	* Returns the list of all registered components
	*/
	getRegisteredComponents(componentObject) {
		const componentsNode = findProperty(componentObject, "components", (p) => p.value.type === "ObjectExpression");
		if (!componentsNode || componentsNode.value.type !== "ObjectExpression") return [];
		return componentsNode.value.properties.filter(isProperty).map((node) => {
			const name = getStaticPropertyName(node);
			return name ? {
				node,
				name
			} : null;
		}).filter(isDef);
	},
	/**
	* Check whether the previous sibling element has `if` or `else-if` directive.
	*/
	prevElementHasIf(node) {
		const prev = this.prevSibling(node);
		return prev != null && prev.startTag.attributes.some((a) => a.directive && (a.key.name.name === "if" || a.key.name.name === "else-if"));
	},
	/**
	* Returns a generator with all child element v-if chains of the given element.
	*/
	*iterateChildElementsChains(node) {
		let isVIf = false;
		let elementChain = [];
		for (const childNode of node.children) if (childNode.type === "VElement") {
			let connected;
			if (hasDirective(childNode, "if")) {
				connected = false;
				isVIf = true;
			} else if (hasDirective(childNode, "else-if")) {
				connected = isVIf;
				isVIf = true;
			} else if (hasDirective(childNode, "else")) {
				connected = isVIf;
				isVIf = false;
			} else {
				connected = false;
				isVIf = false;
			}
			if (connected) elementChain.push(childNode);
			else {
				if (elementChain.length > 0) yield elementChain;
				elementChain = [childNode];
			}
		} else if (childNode.type !== "VText" || childNode.value.trim() !== "") isVIf = false;
		if (elementChain.length > 0) yield elementChain;
	},
	isStringLiteral,
	/**
	* Check whether the given node is a custom component or not.
	*/
	isCustomComponent(node, shouldIgnoreElementNamespaces = false) {
		if (hasAttribute(node, "is") || hasDirective(node, "bind", "is") || hasDirective(node, "is")) return true;
		const isHtmlName = this.isHtmlWellKnownElementName(node.rawName);
		const isSvgName = this.isSvgWellKnownElementName(node.rawName);
		const isMathName = this.isMathWellKnownElementName(node.rawName);
		return shouldIgnoreElementNamespaces ? !isHtmlName && !isSvgName && !isMathName : this.isHtmlElementNode(node) && !isHtmlName || this.isSvgElementNode(node) && !isSvgName || this.isMathElementNode(node) && !isMathName;
	},
	/**
	* Check whether the given node is a HTML element or not.
	*/
	isHtmlElementNode(node) {
		return node.namespace === NS.HTML;
	},
	/**
	* Check whether the given node is a SVG element or not.
	*/
	isSvgElementNode(node) {
		return node.namespace === NS.SVG;
	},
	/**
	* Check whether the given name is a MathML element or not.
	*/
	isMathElementNode(node) {
		return node.namespace === NS.MathML;
	},
	VUE2_BUILTIN_COMPONENT_NAMES,
	VUE3_BUILTIN_COMPONENT_NAMES,
	/**
	* Check whether the given name is an well-known element or not.
	*/
	isHtmlWellKnownElementName(name) {
		return HTML_ELEMENT_NAMES.has(name);
	},
	/**
	* Check whether the given name is an well-known SVG element or not.
	*/
	isSvgWellKnownElementName(name) {
		return SVG_ELEMENT_NAMES.has(name);
	},
	/**
	* Check whether the given name is a well-known MathML element or not.
	*/
	isMathWellKnownElementName(name) {
		return MATH_ELEMENT_NAMES.has(name);
	},
	/**
	* Check whether the given name is a void element name or not.
	*/
	isHtmlVoidElementName(name) {
		return VOID_ELEMENT_NAMES.has(name);
	},
	/**
	* Check whether the given name is Vue builtin component name or not.
	*/
	isBuiltInComponentName(name) {
		return VUE3_BUILTIN_COMPONENT_NAMES.has(name) || VUE2_BUILTIN_COMPONENT_NAMES.has(name);
	},
	/**
	* Check whether the given name is Vue builtin element name or not.
	*/
	isVueBuiltInElementName(name) {
		return VUE_BUILTIN_ELEMENT_NAMES.has(name.toLowerCase());
	},
	/**
	* Check whether the given name is Vue builtin directive name or not.
	*/
	isBuiltInDirectiveName(name) {
		return BUILTIN_DIRECTIVE_NAMES.has(name);
	},
	/**
	* Gets the property name of a given node.
	*/
	getStaticPropertyName,
	/**
	* Gets the string of a given node.
	*/
	getStringLiteralValue,
	/**
	* Gets the name parameter from the given call expression.
	*/
	getNameParamNode,
	/**
	* Get all props by looking at all component's properties
	*/
	getComponentPropsFromOptions,
	/**
	* Get all emits by looking at all component's properties
	*/
	getComponentEmitsFromOptions,
	/**
	* Get all computed properties by looking at all component's properties
	*/
	getComputedProperties(componentObject) {
		const computedPropertiesNode = findProperty(componentObject, "computed", (p) => p.value.type === "ObjectExpression");
		if (!computedPropertiesNode || computedPropertiesNode.value.type !== "ObjectExpression") return [];
		return computedPropertiesNode.value.properties.filter(isProperty).map((cp) => {
			const key = getStaticPropertyName(cp);
			const propValue = skipTSAsExpression(cp.value);
			let value = null;
			if (propValue.type === "FunctionExpression") value = propValue.body;
			else if (propValue.type === "ObjectExpression") {
				const get = findProperty(propValue, "get", (p) => p.value.type === "FunctionExpression");
				value = get ? get.value.body : null;
			}
			return {
				key,
				value
			};
		});
	},
	/**
	* Get getter body from computed function
	*/
	getGetterBodyFromComputedFunction(callExpression) {
		if (callExpression.arguments.length === 0) return null;
		const arg = callExpression.arguments[0];
		if (arg.type === "FunctionExpression" || arg.type === "ArrowFunctionExpression") return arg;
		if (arg.type === "ObjectExpression") {
			const getProperty = findProperty(arg, "get", (p) => p.value.type === "FunctionExpression" || p.value.type === "ArrowFunctionExpression");
			return getProperty ? getProperty.value : null;
		}
		return null;
	},
	isTypeScriptFile,
	isVueFile,
	/**
	* Checks whether the current file is uses `<script setup>`
	*/
	isScriptSetup,
	/**
	* Gets the element of `<script setup>`
	*/
	getScriptSetupElement,
	/**
	* Check if current file is a Vue instance or component and call callback
	*/
	executeOnVue(context, cb) {
		return compositingVisitors(this.executeOnVueComponent(context, cb), this.executeOnVueInstance(context, cb));
	},
	/**
	* Define handlers to traverse the Vue Objects.
	* Some special events are available to visitor.
	*
	* - `onVueObjectEnter` ... Event when Vue Object is found.
	* - `onVueObjectExit` ... Event when Vue Object visit ends.
	* - `onSetupFunctionEnter` ... Event when setup function found.
	* - `onRenderFunctionEnter` ... Event when render function found.
	*/
	defineVueVisitor(context, visitor) {
		let vueStack = null;
		function callVisitor(key, node) {
			if (vueStack && visitor[key]) visitor[key](node, vueStack);
		}
		const vueVisitor = {};
		for (const key in visitor) vueVisitor[key] = (node) => callVisitor(key, node);
		vueVisitor.ObjectExpression = (node) => {
			const type = getVueObjectType(context, node);
			if (type) {
				vueStack = {
					node,
					type,
					parent: vueStack,
					get functional() {
						const functional = findProperty(node, "functional");
						return functional !== null && (functional.value.type !== "Literal" || functional.value.value !== false);
					}
				};
				callVisitor("onVueObjectEnter", node);
			}
			callVisitor("ObjectExpression", node);
		};
		vueVisitor["ObjectExpression:exit"] = (node) => {
			callVisitor("ObjectExpression:exit", node);
			if (!vueStack || vueStack.node !== node) return;
			callVisitor("onVueObjectExit", node);
			vueStack = vueStack.parent;
		};
		if (visitor.onSetupFunctionEnter || visitor.onSetupFunctionExit || visitor.onRenderFunctionEnter) {
			const setups = /* @__PURE__ */ new Set();
			vueVisitor["Property[value.type=/^(Arrow)?FunctionExpression$/] > :function"] = (node) => {
				const prop = node.parent;
				if (vueStack && prop.parent === vueStack.node && prop.value === node) {
					const name = getStaticPropertyName(prop);
					if (name === "setup") {
						callVisitor("onSetupFunctionEnter", node);
						setups.add(node);
					} else if (name === "render") callVisitor("onRenderFunctionEnter", node);
				}
				callVisitor("Property[value.type=/^(Arrow)?FunctionExpression$/] > :function", node);
			};
			if (visitor.onSetupFunctionExit) vueVisitor["Property[value.type=/^(Arrow)?FunctionExpression$/] > :function:exit"] = (node) => {
				if (!setups.has(node)) return;
				callVisitor("onSetupFunctionExit", node);
				setups.delete(node);
			};
		}
		return vueVisitor;
	},
	/**
	* Define handlers to traverse the AST nodes in `<script setup>`.
	* Some special events are available to visitor.
	*
	* - `onDefinePropsEnter` ... Event when defineProps is found.
	* - `onDefinePropsExit` ... Event when defineProps visit ends.
	* - `onDefineEmitsEnter` ... Event when defineEmits is found.
	* - `onDefineEmitsExit` ... Event when defineEmits visit ends.
	* - `onDefineOptionsEnter` ... Event when defineOptions is found.
	* - `onDefineOptionsExit` ... Event when defineOptions visit ends.
	* - `onDefineSlotsEnter` ... Event when defineSlots is found.
	* - `onDefineSlotsExit` ... Event when defineSlots visit ends.
	* - `onDefineExposeEnter` ... Event when defineExpose is found.
	* - `onDefineExposeExit` ... Event when defineExpose visit ends.
	* - `onDefineModelEnter` ... Event when defineModel is found.
	* - `onDefineModelExit` ... Event when defineModel visit ends.
	*/
	defineScriptSetupVisitor(context, visitor) {
		const scriptSetup = getScriptSetupElement(context);
		if (scriptSetup == null) return {};
		const scriptSetupRange = scriptSetup.range;
		function isInScriptSetup(node) {
			return scriptSetupRange[0] <= node.range[0] && node.range[1] <= scriptSetupRange[1];
		}
		function callVisitor(key, node, ...args) {
			if (visitor[key] && (node.type === "Program" || isInScriptSetup(node))) visitor[key](node, ...args);
		}
		const scriptSetupVisitor = {};
		for (const key in visitor) scriptSetupVisitor[key] = (node) => callVisitor(key, node);
		class MacroListener {
			name;
			enterName;
			exitName;
			isMacroNode;
			buildParam;
			hasListener;
			paramsMap;
			constructor(name, enterName, exitName, isMacroNode, buildParam) {
				this.name = name;
				this.enterName = enterName;
				this.exitName = exitName;
				this.isMacroNode = isMacroNode;
				this.buildParam = buildParam;
				this.hasListener = Boolean(visitor[this.enterName] || visitor[this.exitName]);
				this.paramsMap = /* @__PURE__ */ new Map();
			}
		}
		const macroListenerList = [
			new MacroListener("defineProps", "onDefinePropsEnter", "onDefinePropsExit", (candidateMacro, node) => candidateMacro === node || candidateMacro === getWithDefaults(node), getComponentPropsFromDefineProps),
			new MacroListener("defineEmits", "onDefineEmitsEnter", "onDefineEmitsExit", (candidateMacro, node) => candidateMacro === node, getComponentEmitsFromDefineEmits),
			new MacroListener("defineOptions", "onDefineOptionsEnter", "onDefineOptionsExit", (candidateMacro, node) => candidateMacro === node, () => void 0),
			new MacroListener("defineSlots", "onDefineSlotsEnter", "onDefineSlotsExit", (candidateMacro, node) => candidateMacro === node, getComponentSlotsFromDefineSlots),
			new MacroListener("defineExpose", "onDefineExposeEnter", "onDefineExposeExit", (candidateMacro, node) => candidateMacro === node, () => void 0),
			new MacroListener("defineModel", "onDefineModelEnter", "onDefineModelExit", (candidateMacro, node) => candidateMacro === node, getComponentModelFromDefineModel)
		].filter((m) => m.hasListener);
		if (macroListenerList.length > 0) {
			let candidateMacro = null;
			scriptSetupVisitor["Program > VariableDeclaration > VariableDeclarator, Program > ExpressionStatement"] = (node) => {
				if (!candidateMacro) candidateMacro = node.type === "VariableDeclarator" ? node.init : node.expression;
			};
			scriptSetupVisitor["Program > VariableDeclaration > VariableDeclarator, Program > ExpressionStatement:exit"] = (node) => {
				if (candidateMacro === (node.type === "VariableDeclarator" ? node.init : node.expression)) candidateMacro = null;
			};
			scriptSetupVisitor.CallExpression = (node) => {
				if (candidateMacro && isInScriptSetup(node) && node.callee.type === "Identifier") for (const macroListener of macroListenerList) {
					if (node.callee.name !== macroListener.name || !macroListener.isMacroNode(candidateMacro, node)) continue;
					const param = macroListener.buildParam(context, node);
					callVisitor(macroListener.enterName, node, param);
					macroListener.paramsMap.set(node, param);
					break;
				}
				callVisitor("CallExpression", node);
			};
			scriptSetupVisitor["CallExpression:exit"] = (node) => {
				callVisitor("CallExpression:exit", node);
				for (const macroListener of macroListenerList) {
					if (!macroListener.paramsMap.has(node)) continue;
					callVisitor(macroListener.exitName, node, macroListener.paramsMap.get(node));
					macroListener.paramsMap.delete(node);
				}
			};
		}
		return scriptSetupVisitor;
	},
	/**
	* Checks whether given defineProps call node has withDefaults.
	*/
	hasWithDefaults,
	/**
	* Gets a map of the expressions defined in withDefaults.
	* `withDefaults()` accepts any expression as its defaults argument: the SFC
	* compiler inlines an object literal and falls back to the `mergeDefaults()`
	* runtime helper for everything else. References to a `const` object and
	* spread elements are resolved, but anything that cannot be determined
	* statically sets `hasUnknown`, which means that props missing from `props`
	* may still have a default value.
	*/
	getWithDefaultsPropExpressions,
	/**
	* Gets a map of the property nodes defined in withDefaults.
	* Only an inline object literal is looked at, because the returned nodes are
	* used as edit targets by fixers.
	*/
	getWithDefaultsProps,
	/**
	* Gets the default definition nodes for defineProp
	* using the props destructure with assignment pattern.
	*/
	getDefaultPropExpressionsForPropsDestructure,
	getLeftOfDefineProps,
	/**
	* Checks whether the given defineProps node is using Props Destructure.
	*/
	isUsingPropsDestructure(node) {
		return getLeftOfDefineProps(node)?.type === "ObjectPattern";
	},
	/**
	* Gets the props destructure property nodes for defineProp.
	*/
	getPropsDestructure,
	getVueObjectType,
	getVueComponentDefinitionType,
	/**
	* Checks whether the given object is an SFC definition.
	*/
	isSFCObject,
	compositingVisitors,
	/**
	* Check if current file is a Vue instance (new Vue) and call callback
	*/
	executeOnVueInstance(context, cb) {
		return { "ObjectExpression:exit"(node) {
			const type = getVueObjectType(context, node);
			if (!type || type !== "instance") return;
			cb(node, type);
		} };
	},
	/**
	* Check if current file is a Vue component and call callback
	*/
	executeOnVueComponent(context, cb) {
		const types = /* @__PURE__ */ new Set([
			"mark",
			"export",
			"definition"
		]);
		return { "ObjectExpression:exit"(node) {
			const type = getVueObjectType(context, node);
			if (type && types.has(type)) cb(node, type);
		} };
	},
	/**
	* Check call `Vue.component` and call callback.
	*/
	executeOnCallVueComponent(_context, cb) {
		return { "CallExpression > MemberExpression > Identifier[name='component']": (node) => {
			const callExpr = node.parent.parent;
			const callee = callExpr.callee;
			if (callee.type !== "MemberExpression") return;
			if (skipTSAsExpression(callee.object).type === "Identifier" && callee.property === node && callExpr.arguments.length > 0) cb(callExpr);
		} };
	},
	/**
	* Return generator with all properties
	*/
	*iterateProperties(node, groups) {
		for (const item of node.properties) {
			if (item.type !== "Property") continue;
			const name = getStaticPropertyName(item);
			if (!name || !groups.has(name)) continue;
			yield* this.iteratePropertyValue(item.value, name);
		}
	},
	/**
	* Return generator with all properties defined by the given property value.
	*/
	*iteratePropertyValue(value, groupName) {
		switch (value.type) {
			case "ArrayExpression":
				yield* this.iterateArrayExpression(value, groupName);
				break;
			case "ObjectExpression":
				yield* this.iterateObjectExpression(value, groupName);
				break;
			case "FunctionExpression":
				yield* this.iterateFunctionExpression(value, groupName);
				break;
			case "ArrowFunctionExpression": yield* this.iterateArrowFunctionExpression(value, groupName);
		}
	},
	/**
	* Return generator with all elements inside ArrayExpression
	*/
	*iterateArrayExpression(node, groupName) {
		for (const item of node.elements) {
			if (!item || item.type !== "Literal" && item.type !== "TemplateLiteral") continue;
			const name = getStringLiteralValue(item);
			if (name) yield {
				type: "array",
				name,
				groupName,
				node: item
			};
		}
	},
	/**
	* Return generator with all elements inside ObjectExpression
	*/
	*iterateObjectExpression(node, groupName) {
		let usedGetter;
		for (const item of node.properties) {
			if (item.type !== "Property") continue;
			const key = item.key;
			if (key.type !== "Identifier" && key.type !== "Literal" && key.type !== "TemplateLiteral") continue;
			const name = getStaticPropertyName(item);
			if (!name) continue;
			if (item.kind === "set" && node.properties.some((item2) => {
				if (item2.type === "Property" && item2.kind === "get") {
					if (!usedGetter) usedGetter = /* @__PURE__ */ new Set();
					if (usedGetter.has(item2)) return false;
					if (getStaticPropertyName(item2) === name) {
						usedGetter.add(item2);
						return true;
					}
				}
				return false;
			})) continue;
			yield {
				type: "object",
				name,
				groupName,
				node: key,
				property: item
			};
		}
	},
	/**
	* Return generator with all elements inside FunctionExpression
	*/
	*iterateFunctionExpression(node, groupName) {
		if (node.body.type === "BlockStatement") {
			for (const item of node.body.body) if (item.type === "ReturnStatement" && item.argument && item.argument.type === "ObjectExpression") yield* this.iterateObjectExpression(item.argument, groupName);
		}
	},
	/**
	* Return generator with all elements inside ArrowFunctionExpression
	*/
	*iterateArrowFunctionExpression(node, groupName) {
		const body = node.body;
		if (body.type === "BlockStatement") {
			for (const item of body.body) if (item.type === "ReturnStatement" && item.argument && item.argument.type === "ObjectExpression") yield* this.iterateObjectExpression(item.argument, groupName);
		} else if (body.type === "ObjectExpression") yield* this.iterateObjectExpression(body, groupName);
	},
	/**
	* Find all functions which do not always return values
	*/
	executeOnFunctionsWithoutReturn(context, shouldTreatUndefinedAsUnspecified, cb) {
		let funcInfo = null;
		const spuriousSegments = /* @__PURE__ */ new WeakSet();
		const caseBodySegments = /* @__PURE__ */ new WeakSet();
		let caseBodyDepth = 0;
		/**
		* Treat spurious segments the same as unreachable.
		*/
		function isEffectivelyUnreachable(segment) {
			return !segment.reachable || spuriousSegments.has(segment);
		}
		function isValidReturn() {
			return !funcInfo || funcInfo.currentSegments.every((segment) => isEffectivelyUnreachable(segment)) && (!shouldTreatUndefinedAsUnspecified || funcInfo.hasReturnValue);
		}
		return {
			onCodePathStart(codePath, node) {
				if (node.type === "ArrowFunctionExpression" || node.type === "FunctionExpression" || node.type === "FunctionDeclaration") funcInfo = {
					codePath,
					currentSegments: [],
					funcInfo,
					hasReturn: false,
					hasReturnValue: false,
					node
				};
			},
			onCodePathSegmentStart(segment) {
				funcInfo?.currentSegments.push(segment);
				if (segment.prevSegments.length > 0 && segment.prevSegments.every(isEffectivelyUnreachable)) spuriousSegments.add(segment);
				if (caseBodyDepth > 0) caseBodySegments.add(segment);
			},
			onCodePathSegmentEnd() {
				funcInfo?.currentSegments.pop();
			},
			onCodePathEnd() {
				funcInfo &&= funcInfo.funcInfo;
			},
			ReturnStatement(node) {
				if (!funcInfo) return;
				funcInfo.hasReturn = true;
				funcInfo.hasReturnValue = Boolean(node.argument);
			},
			SwitchCase() {
				caseBodyDepth++;
			},
			"SwitchCase:exit"() {
				caseBodyDepth--;
			},
			"SwitchStatement:exit"(node) {
				if (!funcInfo || node.cases.some((switchCase) => switchCase.test === null) || !require_ts_types.isExhaustiveSwitch(context, node)) return;
				if (funcInfo.currentSegments.some((post) => post.prevSegments.some((prev) => !isEffectivelyUnreachable(prev) && caseBodySegments.has(prev)))) return;
				for (const segment of funcInfo.currentSegments) spuriousSegments.add(segment);
			},
			"ArrowFunctionExpression:exit"(node) {
				if (funcInfo && !isValidReturn() && !node.expression) cb(funcInfo.node);
			},
			"FunctionExpression:exit"() {
				if (funcInfo && !isValidReturn()) cb(funcInfo.node);
			}
		};
	},
	/**
	* Check whether the component is declared in a single line or not.
	*/
	isSingleLine(node) {
		return node.loc.start.line === node.loc.end.line;
	},
	/**
	* Check whether the templateBody of the program has invalid EOF or not.
	*/
	hasInvalidEOF(node) {
		const body = node.templateBody;
		return body != null && body.errors != null && body.errors.some((error) => typeof error.code === "string" && error.code.startsWith("eof-"));
	},
	/**
	* Get the chaining nodes of MemberExpression.
	*/
	getMemberChaining(node) {
		const nodes = [];
		let n = skipChainExpression(node);
		while (n.type === "MemberExpression") {
			nodes.push(n);
			n = skipChainExpression(n.object);
		}
		return [n, ...nodes.reverse()];
	},
	/**
	* return two string editdistance
	*/
	editDistance(a, b) {
		if (a === b) return 0;
		const alen = a.length;
		const blen = b.length;
		const dp = Array.from({ length: alen + 1 }, () => Array.from({ length: blen + 1 }, () => 0));
		for (let i = 0; i <= alen; i++) dp[i][0] = i;
		for (let j = 0; j <= blen; j++) dp[0][j] = j;
		for (let i = 1; i <= alen; i++) for (let j = 1; j <= blen; j++) dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] : Math.min(dp[i - 1][j], dp[i][j - 1], dp[i - 1][j - 1]) + 1;
		return dp[alen][blen];
	},
	/**
	* Checks whether the target node is within the given range.
	*/
	isInRange(rangeOrNode, target) {
		const range = Array.isArray(rangeOrNode) ? rangeOrNode : rangeOrNode.range;
		return range[0] <= target.range[0] && target.range[1] <= range[1];
	},
	/**
	* Checks whether the given node is Property.
	*/
	isProperty,
	/**
	* Checks whether the given node is AssignmentProperty.
	*/
	isAssignmentProperty,
	/**
	* Checks whether the given node is VElement.
	*/
	isVElement,
	/**
	* Finds the property with the given name from the given ObjectExpression node.
	*/
	findProperty,
	/**
	* Finds the assignment property with the given name from the given ObjectPattern node.
	*/
	findAssignmentProperty,
	/**
	* Checks if the given node is a property value.
	*/
	isPropertyChain,
	/**
	* Retrieve `TSAsExpression#expression` value if the given node a `TSAsExpression` node. Otherwise, pass through it.
	*/
	skipTSAsExpression,
	/**
	* Retrieve `AssignmentPattern#left` value if the given node a `AssignmentPattern` node. Otherwise, pass through it.
	*/
	skipDefaultParamValue,
	/**
	* Retrieve `ChainExpression#expression` value if the given node a `ChainExpression` node. Otherwise, pass through it.
	*/
	skipChainExpression,
	/**
	* Checks whether the given node is in a type annotation.
	*/
	isWithinTypeNode,
	findVariableByIdentifier,
	getScope: require_scope.getScope,
	/**
	* Checks whether the given node is in export default.
	*/
	isInExportDefault,
	/**
	* Check whether the given node is `this` or variable that stores `this`.
	*/
	isThis(node, context) {
		if (node.type === "ThisExpression") return true;
		if (node.type !== "Identifier") return false;
		const parent = node.parent;
		if (parent.type === "MemberExpression" && parent.property === node || parent.type === "Property" && parent.key === node && !parent.computed) return false;
		const variable = (0, _eslint_community_eslint_utils.findVariable)(require_scope.getScope(context, node), node);
		if (variable != null && variable.defs.length === 1) {
			const def = variable.defs[0];
			if (def.type === "Variable" && def.parent.kind === "const" && def.node.id.type === "Identifier") return Boolean(def.node && def.node.init && def.node.init.type === "ThisExpression");
		}
		return false;
	},
	/**
	* Check if a call expression is Object.assign with the given node as first argument
	*/
	isObjectAssignCall(callExpr, targetNode) {
		const { callee, arguments: args } = callExpr;
		return args.length > 0 && args[0] === targetNode && callee?.type === "MemberExpression" && callee.object?.type === "Identifier" && callee.object.name === "Object" && callee.property?.type === "Identifier" && callee.property.name === "assign";
	},
	findMutating(props) {
		const pathNodes = [];
		let node = props;
		let target = node.parent;
		while (true) {
			const step = this.findMutatingStep(node, target, pathNodes);
			if (step.done) return step.result;
			node = step.node;
			target = step.target;
		}
	},
	/**
	* Resolve a single step of the parent chain walk in `findMutating`.
	*/
	findMutatingStep(node, target, pathNodes) {
		switch (target.type) {
			case "AssignmentExpression":
				if (target.left === node) return {
					done: true,
					result: {
						kind: "assignment",
						node: target,
						pathNodes
					}
				};
				return {
					done: true,
					result: null
				};
			case "UpdateExpression": return {
				done: true,
				result: {
					kind: "update",
					node: target,
					pathNodes
				}
			};
			case "UnaryExpression":
				if (target.operator === "delete") return {
					done: true,
					result: {
						kind: "update",
						node: target,
						pathNodes
					}
				};
				return {
					done: true,
					result: null
				};
			case "CallExpression": {
				const mem = pathNodes.at(-1);
				if (mem && target.callee === node) {
					const callName = getStaticPropertyName(mem);
					if (callName && /^(?:push|pop|shift|unshift|reverse|splice|sort|copyWithin|fill)$/u.test(callName)) {
						pathNodes.pop();
						return {
							done: true,
							result: {
								kind: "call",
								node: target,
								pathNodes
							}
						};
					}
				}
				if (this.isObjectAssignCall(target, node)) return {
					done: true,
					result: {
						kind: "call",
						node: target,
						pathNodes
					}
				};
				return {
					done: true,
					result: null
				};
			}
			case "MemberExpression":
				if (target.object === node) {
					pathNodes.push(target);
					return {
						done: false,
						node: target,
						target: target.parent
					};
				}
				return {
					done: true,
					result: null
				};
			case "ChainExpression": return {
				done: false,
				node: target,
				target: target.parent
			};
			default: return {
				done: true,
				result: null
			};
		}
	},
	/**
	* Return generator with the all handler nodes defined in the given watcher property.
	*/
	iterateWatchHandlerValues,
	/**
	* Wraps composition API trace map in 'vue', '@vue/composition-api' and '@nuxtjs/composition-api' imports, or '#imports' from unimport
	*/
	createCompositionApiTraceMap,
	/**
	* Iterates all references in the given trace map.
	* Take the third argument option to detect auto-imported references.
	*/
	*iterateReferencesTraceMap(tracker, map) {
		const esmTraceMap = this.createCompositionApiTraceMap({
			...map,
			[_eslint_community_eslint_utils.ReferenceTracker.ESM]: true
		});
		for (const ref of tracker.iterateEsmReferences(esmTraceMap)) yield ref;
		for (const ref of tracker.iterateGlobalReferences(map)) yield ref;
	},
	/**
	* Checks whether or not the tokens of two given nodes are same.
	*/
	equalTokens(left, right, sourceCode) {
		const tokensL = sourceCode.getTokens(left);
		const tokensR = sourceCode.getTokens(right);
		return tokensL.length === tokensR.length && tokensL.every((token, i) => token.type === tokensR[i].type && token.value === tokensR[i].value);
	}
};
const { defineScriptSetupVisitor, editDistance, executeOnVueComponent, getComputedProperties, getMemberChaining, getRegisteredComponents } = utils;
/**
* Checks whether the given value is defined.
*/
function isDef(v) {
	return v != null;
}
/**
* Flattens arrays, objects and iterable objects.
*/
function flatten(v) {
	const result = [];
	if (v) {
		if (isIterable(v)) result.push(...v);
		else result.push(v);
	}
	return result;
}
function isIterable(v) {
	return v && Symbol.iterator in v;
}
/**
* Register the given visitor to parser services.
* If the parser service of `vue-eslint-parser` was not found,
* this generates a warning.
*/
function defineTemplateBodyVisitor(context, templateBodyVisitor, scriptVisitor, options) {
	const sourceCode = context.sourceCode;
	if (sourceCode.parserServices.defineTemplateBodyVisitor == null) {
		const filename = context.filename;
		if (node_path.default.extname(filename) === ".vue") context.report({
			loc: {
				line: 1,
				column: 0
			},
			message: "Use the latest vue-eslint-parser. See also https://eslint.vuejs.org/user-guide/#what-is-the-use-the-latest-vue-eslint-parser-error."
		});
		return {};
	}
	return sourceCode.parserServices.defineTemplateBodyVisitor(templateBodyVisitor, scriptVisitor, options);
}
/**
* Register the given visitor to parser services.
* If the parser service of `vue-eslint-parser` was not found,
* this generates a warning.
*/
function defineDocumentVisitor(context, documentVisitor, options) {
	const sourceCode = context.sourceCode;
	if (sourceCode.parserServices.defineDocumentVisitor == null) {
		const filename = context.filename;
		if (node_path.default.extname(filename) === ".vue") context.report({
			loc: {
				line: 1,
				column: 0
			},
			message: "Use the latest vue-eslint-parser. See also https://eslint.vuejs.org/user-guide/#what-is-the-use-the-latest-vue-eslint-parser-error."
		});
		return {};
	}
	return sourceCode.parserServices.defineDocumentVisitor(documentVisitor, options);
}
function compositingVisitors(visitor, ...visitors) {
	for (const v of visitors) for (const key in v) if (visitor[key]) {
		const o = visitor[key];
		visitor[key] = (...args) => {
			o(...args);
			v[key](...args);
		};
	} else visitor[key] = v[key];
	return visitor;
}
/**
* Find the variable of a given identifier.
*/
function findVariableByIdentifier(context, node) {
	return (0, _eslint_community_eslint_utils.findVariable)(require_scope.getScope(context, node), node);
}
/**
* Finds the property with the given name from the given ObjectExpression node.
*/
function findProperty(node, name, isMatching) {
	return node.properties.find((prop) => isProperty(prop) && getStaticPropertyName(prop) === name && (isMatching?.(prop) ?? true)) ?? null;
}
/**
* Finds the assignment property with the given name from the given ObjectPattern node.
*/
function findAssignmentProperty(node, name, isMatching) {
	return node.properties.find((prop) => isAssignmentProperty(prop) && getStaticPropertyName(prop) === name && (isMatching?.(prop) ?? true)) ?? null;
}
/**
* Checks whether the given node is Property.
*/
function isProperty(node) {
	return node.type === "Property" && (!node.parent || node.parent.type === "ObjectExpression");
}
/**
* Checks whether the given node is AssignmentProperty.
*/
function isAssignmentProperty(node) {
	return node.type === "Property";
}
/**
* Checks whether the given node is VElement.
*/
function isVElement(node) {
	return node.type === "VElement";
}
/**
* Checks whether the given node is in export default.
*/
function isInExportDefault(node) {
	let parent = node.parent;
	while (parent) {
		if (parent.type === "ExportDefaultDeclaration") return true;
		parent = parent.parent;
	}
	return false;
}
/**
* Retrieve `TSAsExpression#expression` value if the given node a `TSAsExpression` node. Otherwise, pass through it.
*/
function skipTSAsExpression(node) {
	if (!node) return node;
	return node.type === "TSAsExpression" ? skipTSAsExpression(node.expression) : node;
}
/**
* Gets the parent node of the given node. This method returns a value ignoring `X as F`.
*/
function getParent(node) {
	let parent = node.parent;
	while (parent.type === "TSAsExpression") parent = parent.parent;
	return parent;
}
/**
* Checks if the given node is a property value.
*/
function isPropertyChain(prop, node) {
	let value = node;
	while (value.parent.type === "TSAsExpression") value = value.parent;
	return prop === value.parent && prop.value === value;
}
/**
* Retrieve `AssignmentPattern#left` value if the given node a `AssignmentPattern` node. Otherwise, pass through it.
*/
function skipDefaultParamValue(node) {
	if (!node) return node;
	return node.type === "AssignmentPattern" ? skipDefaultParamValue(node.left) : node;
}
/**
* Retrieve `ChainExpression#expression` value if the given node a `ChainExpression` node. Otherwise, pass through it.
*/
function skipChainExpression(node) {
	if (!node) return node;
	return node.type === "ChainExpression" ? skipChainExpression(node.expression) : node;
}
/**
* Checks whether the given node is in a type annotation.
*/
function isWithinTypeNode(node) {
	let target = node;
	while (target) {
		if (require_ts_ast.isTypeNode(target)) return true;
		target = target.parent;
	}
	return false;
}
/**
* Gets the property name of a given node.
*/
function getStaticPropertyName(node) {
	if (node.type === "Property" || node.type === "MethodDefinition") {
		if (!node.computed) {
			const key = node.key;
			if (key.type === "Identifier") return key.name;
		}
		const key = node.key;
		return getStringLiteralValue(key);
	}
	if (node.type === "MemberExpression") {
		if (!node.computed) {
			const property = node.property;
			return property.type === "Identifier" ? property.name : null;
		}
		const property = node.property;
		return getStringLiteralValue(property);
	}
	return null;
}
/**
* Gets the string of a given node.
*/
function getStringLiteralValue(node, shouldOnlyAcceptStrings = false) {
	if (node.type === "Literal") {
		if (node.value == null) return shouldOnlyAcceptStrings || node.bigint == null ? null : node.bigint;
		if (typeof node.value === "string") return node.value;
		return shouldOnlyAcceptStrings ? null : String(node.value);
	}
	return node.type === "TemplateLiteral" && node.expressions.length === 0 && node.quasis.length === 1 ? node.quasis[0].value.cooked : null;
}
/**
* Gets the name parameter from the given call expression.
*/
function getNameParamNode(node) {
	const nameLiteralNode = node.arguments[0];
	if (nameLiteralNode && isStringLiteral(nameLiteralNode)) {
		const name = getStringLiteralValue(nameLiteralNode);
		if (name != null) return {
			name,
			loc: nameLiteralNode.loc,
			range: nameLiteralNode.range
		};
	}
	return null;
}
/**
* Gets the VExpressionContainer of a given node.
*/
function getVExpressionContainer(node) {
	let n = node;
	while (n && n.type !== "VExpressionContainer") n = n.parent;
	return n;
}
function isTypeScriptFile(path) {
	return path.endsWith(".ts") || path.endsWith(".tsx") || path.endsWith(".mts");
}
function isVueFile(path) {
	return path.endsWith(".vue") || path.endsWith(".jsx") || path.endsWith(".tsx");
}
/**
* Checks whether the current file is uses `<script setup>`
*/
function isScriptSetup(context) {
	return Boolean(getScriptSetupElement(context));
}
/**
* Gets the element of `<script setup>`
*/
function getScriptSetupElement(context) {
	const sourceCode = context.sourceCode;
	const df = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
	if (!df) return null;
	const scripts = df.children.filter((e) => isVElement(e) && e.name === "script");
	if (scripts.length === 2) return scripts.find((e) => hasAttribute(e, "setup")) || null;
	const script = scripts[0];
	return script && hasAttribute(script, "setup") ? script : null;
}
/**
* Check whether the given node is a Vue component based
* on the filename and default export type
* export default {} in .vue || .jsx
*/
function isVueComponentFile(node, path) {
	return isVueFile(path) && node.type === "ExportDefaultDeclaration" && node.declaration.type === "ObjectExpression";
}
/**
* Whether the given callee refers to a symbol exported by Vue.
*/
function isGlobalOrImportedFromVue(callee, context) {
	const globalScope = context.sourceCode.scopeManager.globalScope;
	if (globalScope === null) return true;
	const tracker = new _eslint_community_eslint_utils.ReferenceTracker(globalScope);
	const globalTraceMap = { [callee.name]: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true } };
	const cjsTraceMap = createCompositionApiTraceMap(globalTraceMap);
	const esmTraceMap = createCompositionApiTraceMap({
		[_eslint_community_eslint_utils.ReferenceTracker.ESM]: true,
		[callee.name]: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true }
	});
	function* allReferences() {
		yield* tracker.iterateGlobalReferences(globalTraceMap);
		yield* tracker.iterateEsmReferences(esmTraceMap);
		yield* tracker.iterateCjsReferences(cjsTraceMap);
	}
	for (const { node } of allReferences()) if (node === callee.parent) return true;
	const variable = (0, _eslint_community_eslint_utils.findVariable)(require_scope.getScope(context, callee), callee);
	if (variable !== null && variable.defs.length === 1) {
		const def = variable.defs[0];
		const vueLikePackageNames = /* @__PURE__ */ new Set([
			"vue",
			"@vue/composition-api",
			"@nuxtjs/composition-api"
		]);
		if (def.type === "ImportBinding" && def.node.type === "ImportSpecifier" && def.node.imported.type === "Identifier" && def.node.parent.type === "ImportDeclaration") return vueLikePackageNames.has(def.node.parent.source.value);
		if (def.type === "Variable" && def.node.id.type === "ObjectPattern" && def.node.init !== null) {
			const init = def.node.init;
			if (init.type === "CallExpression") {
				const firstArg = init.arguments[0];
				if (init.callee.type === "Identifier" && init.callee.name === "require") {
					const source = isStringLiteral(firstArg) ? getStringLiteralValue(firstArg) : null;
					return vueLikePackageNames.has(source);
				}
			}
		}
		if ([
			"CatchClause",
			"ClassName",
			"FunctionName",
			"TDZ"
		].includes(def.type)) return false;
	}
	return true;
}
/**
* Get the Vue component definition type from given node
* Vue.component('xxx', {}) || component('xxx', {})
*/
function getVueComponentDefinitionType(context, node) {
	const parent = getParent(node);
	if (parent.type === "CallExpression") {
		const callee = parent.callee;
		if (callee.type === "MemberExpression") {
			const calleeObject = skipTSAsExpression(callee.object);
			if (calleeObject.type === "Identifier") {
				const propName = getStaticPropertyName(callee);
				if (calleeObject.name === "Vue") return propName && isObjectArgument(parent) && (propName === "component" || propName === "mixin" || propName === "extend") ? propName : null;
				return propName && isObjectArgument(parent) && (propName === "component" || propName === "mixin") ? propName : null;
			}
		} else if (callee.type === "Identifier") {
			if (callee.name === "defineNuxtComponent") return isObjectArgument(parent) ? "defineNuxtComponent" : null;
			if ((callee.name === "component" || callee.name === "createApp" || callee.name === "defineComponent") && isGlobalOrImportedFromVue(callee, context)) return isObjectArgument(parent) ? callee.name : null;
		}
	}
	return null;
}
function isObjectArgument(node) {
	const lastArgument = node.arguments.at(-1);
	return lastArgument && skipTSAsExpression(lastArgument).type === "ObjectExpression";
}
/**
* Check whether given node is new Vue instance
* new Vue({})
*/
function isVueInstance(node) {
	const callee = node.callee;
	return node.type === "NewExpression" && callee.type === "Identifier" && callee.name === "Vue" && node.arguments.length > 0 && skipTSAsExpression(node.arguments[0]).type === "ObjectExpression";
}
/**
* If the given object is a Vue component or instance, returns the Vue definition type.
*/
function getVueObjectType(context, node) {
	if (node.type !== "ObjectExpression") return null;
	const parent = getParent(node);
	switch (parent.type) {
		case "ExportDefaultDeclaration": {
			const filePath = context.filename;
			if (isVueComponentFile(parent, filePath) && skipTSAsExpression(parent.declaration) === node) {
				const scriptSetup = getScriptSetupElement(context);
				return scriptSetup && scriptSetup.range[0] <= parent.range[0] && parent.range[1] <= scriptSetup.range[1] ? null : "export";
			}
			break;
		}
		case "CallExpression":
			if (getVueComponentDefinitionType(context, node) != null && skipTSAsExpression(parent.arguments.at(-1)) === node) return "definition";
			break;
		case "NewExpression": if (isVueInstance(parent) && skipTSAsExpression(parent.arguments[0]) === node) return "instance";
	}
	return getComponentComments(context).some((el) => el.loc.end.line === node.loc.start.line - 1) ? "mark" : null;
}
/**
* Checks whether the given object is an SFC definition.
*/
function isSFCObject(context, node) {
	if (node.type !== "ObjectExpression") return false;
	const filePath = context.filename;
	const ext = node_path.default.extname(filePath);
	return (ext === ".vue" || !ext) && isSFC(node);
	function isSFC(node) {
		const parent = getParent(node);
		switch (parent.type) {
			case "ExportDefaultDeclaration": {
				if (skipTSAsExpression(parent.declaration) !== node) return false;
				const scriptSetup = getScriptSetupElement(context);
				return !scriptSetup || scriptSetup.range[0] > parent.range[0] || parent.range[1] > scriptSetup.range[1];
			}
			case "CallExpression": {
				if (parent.arguments.every((arg) => skipTSAsExpression(arg) !== node)) return false;
				const { callee } = parent;
				return (callee.type === "Identifier" && (callee.name === "defineComponent" || callee.name === "defineNuxtComponent") || callee.type === "MemberExpression" && callee.object.type === "Identifier" && callee.object.name === "Vue" && callee.property.type === "Identifier" && callee.property.name === "extend") && isSFC(parent);
			}
			case "VariableDeclarator": {
				if (skipTSAsExpression(parent.init) !== node || parent.id.type !== "Identifier") return false;
				const variable = (0, _eslint_community_eslint_utils.findVariable)(require_scope.getScope(context, node), parent.id);
				return Boolean(variable?.references.some((ref) => isSFC(ref.identifier)));
			}
		}
		return false;
	}
}
/**
* Gets the component comments of a given context.
*/
function getComponentComments(context) {
	let tokens = componentComments.get(context);
	if (tokens) return tokens;
	tokens = context.sourceCode.getAllComments().filter((comment) => /@vue\/component/g.test(comment.value));
	componentComments.set(context, tokens);
	return tokens;
}
/**
* Return generator with the all handler nodes defined in the given watcher property.
*/
function* iterateWatchHandlerValues(property) {
	const value = property.type === "Property" ? property.value : property;
	if (value.type === "ObjectExpression") {
		const handler = findProperty(value, "handler");
		if (handler) yield handler.value;
	} else if (value.type === "ArrayExpression") {
		for (const element of value.elements) if (isDef(element) && element.type !== "SpreadElement") yield* iterateWatchHandlerValues(element);
	} else yield value;
}
/**
* Wraps composition API trace map in 'vue', '@vue/composition-api' and '@nuxtjs/composition-api' imports, or '#imports' from unimport
*/
function createCompositionApiTraceMap(map) {
	return {
		vue: map,
		"@vue/composition-api": map,
		"@nuxtjs/composition-api": map,
		"#imports": map
	};
}
/**
* Get the attribute which has the given name.
*/
function getAttribute(node, name, value) {
	return node.startTag.attributes.find((node) => !node.directive && node.key.name === name && (value === void 0 || node.value != null && node.value.value === value)) || null;
}
/**
* Get the directive list which has the given name.
*/
function getDirectives(node, name) {
	return (node.type === "VElement" ? node.startTag : node).attributes.filter((node) => node.directive && node.key.name.name === name);
}
/**
* Get the directive which has the given name.
*/
function getDirective(node, name, argument) {
	return node.startTag.attributes.find((node) => node.directive && node.key.name.name === name && (argument === void 0 || (node.key.argument && node.key.argument.type === "VIdentifier" && node.key.argument.name) === argument)) || null;
}
/**
* Check whether the given start tag has specific attribute.
*/
function hasAttribute(node, name, value) {
	return Boolean(getAttribute(node, name, value));
}
/**
* Check whether the given start tag has specific directive.
*/
function hasDirective(node, name, argument) {
	return Boolean(getDirective(node, name, argument));
}
/**
* Check whether the given directive node is v-bind same-name shorthand.
*/
function isVBindSameNameShorthand(node) {
	return node.directive && node.key.name.name === "bind" && node.value?.expression?.type === "Identifier" && node.key.range[0] <= node.value.range[0] && node.value.range[1] <= node.key.range[1];
}
function isStringLiteral(node) {
	return node.type === "Literal" && typeof node.value === "string" || node.type === "TemplateLiteral" && node.expressions.length === 0;
}
/**
* Checks whether given defineProps call node has withDefaults.
*/
function hasWithDefaults(node) {
	return node.parent && node.parent.type === "CallExpression" && node.parent.arguments[0] === node && node.parent.callee.type === "Identifier" && node.parent.callee.name === "withDefaults";
}
/**
* Get the withDefaults call node from given defineProps call node.
*/
function getWithDefaults(node) {
	return hasWithDefaults(node) ? node.parent : null;
}
/**
* Gets a map of the property nodes defined in withDefaults.
*/
function getWithDefaultsProps(node) {
	if (!hasWithDefaults(node)) return {};
	const param = node.parent.arguments[1];
	if (!param || param.type !== "ObjectExpression") return {};
	const result = {};
	for (const prop of param.properties) {
		if (prop.type !== "Property") continue;
		const name = getStaticPropertyName(prop);
		if (name != null) result[name] = prop;
	}
	return result;
}
/**
* Collects the property expressions of the object that the given node refers
* to, following `const` references and spread elements. Properties are
* collected in source order, so a later definition overrides an earlier one.
* Returns `false` if the object cannot be determined statically.
*/
function collectPropExpressions(context, node, result, visited) {
	const object = getObjectOrArray(context, node);
	if (object?.type !== "ObjectExpression") return false;
	if (visited.has(object)) return true;
	visited.add(object);
	let isResolved = true;
	for (const property of object.properties) {
		if (property.type === "SpreadElement") {
			if (!collectPropExpressions(context, property.argument, result, visited)) isResolved = false;
			continue;
		}
		const name = getStaticPropertyName(property);
		if (name == null) {
			isResolved = false;
			continue;
		}
		result[name] = property.value;
	}
	return isResolved;
}
/**
* Gets a map of the expressions defined in withDefaults.
*/
function getWithDefaultsPropExpressions(context, node) {
	const props = {};
	if (!hasWithDefaults(node)) return {
		props,
		hasUnknown: false
	};
	const defaults = node.parent.arguments[1];
	if (!defaults) return {
		props,
		hasUnknown: false
	};
	return {
		props,
		hasUnknown: !collectPropExpressions(context, defaults, props, /* @__PURE__ */ new Set())
	};
}
/**
* Gets the props destructure property nodes for defineProp.
*/
function getPropsDestructure(node) {
	const result = Object.create(null);
	const left = getLeftOfDefineProps(node);
	if (!left || left.type !== "ObjectPattern") return result;
	for (const prop of left.properties) {
		if (prop.type !== "Property") continue;
		const name = getStaticPropertyName(prop);
		if (name != null) result[name] = prop;
	}
	return result;
}
/**
* Gets the default definition nodes for defineProp
* using the props destructure with assignment pattern.
*/
function getDefaultPropExpressionsForPropsDestructure(node) {
	const result = Object.create(null);
	const propsDestructureEntries = Object.entries(getPropsDestructure(node));
	for (const [name, prop] of propsDestructureEntries) {
		if (!prop) continue;
		const value = prop.value;
		if (value.type !== "AssignmentPattern") continue;
		result[name] = {
			prop,
			expression: value.right
		};
	}
	return result;
}
/**
* Gets the pattern of the left operand of defineProps.
*/
function getLeftOfDefineProps(node) {
	let target = node;
	if (hasWithDefaults(target)) target = target.parent;
	if (!target.parent) return null;
	return target.parent.type === "VariableDeclarator" && target.parent.init === target ? target.parent.id : null;
}
/**
* Get all props from component options object.
*/
function getComponentPropsFromOptions(componentObject) {
	const propsNode = findProperty(componentObject, "props");
	if (!propsNode) return [];
	if (propsNode.value.type !== "ObjectExpression" && propsNode.value.type !== "ArrayExpression") return [{
		type: "unknown",
		propName: null,
		node: propsNode.value
	}];
	return getComponentPropsFromDefine(propsNode.value);
}
/**
* Get all emits from component options object.
*/
function getComponentEmitsFromOptions(componentObject) {
	const emitsNode = findProperty(componentObject, "emits");
	if (!emitsNode) return [];
	if (emitsNode.value.type !== "ObjectExpression" && emitsNode.value.type !== "ArrayExpression") return [{
		type: "unknown",
		emitName: null,
		node: emitsNode.value
	}];
	return getComponentEmitsFromDefine(emitsNode.value);
}
/**
* Get all props from `defineProps` call expression.
*/
function getComponentPropsFromDefineProps(context, node) {
	if (node.arguments.length > 0) {
		const defNode = getObjectOrArray(context, node.arguments[0]);
		if (defNode) return getComponentPropsFromDefine(defNode);
		return [{
			type: "unknown",
			propName: null,
			node: node.arguments[0]
		}];
	}
	const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
	if (typeArguments && typeArguments.params.length > 0) return require_index.getComponentPropsFromTypeDefine(context, typeArguments.params[0]);
	return [{
		type: "unknown",
		propName: null,
		node: null
	}];
}
/**
* Get all emits from `defineEmits` call expression.
*/
function getComponentEmitsFromDefineEmits(context, node) {
	if (node.arguments.length > 0) {
		const defNode = getObjectOrArray(context, node.arguments[0]);
		if (defNode) return getComponentEmitsFromDefine(defNode);
		return [{
			type: "unknown",
			emitName: null,
			node: node.arguments[0]
		}];
	}
	const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
	if (typeArguments && typeArguments.params.length > 0) return require_index.getComponentEmitsFromTypeDefine(context, typeArguments.params[0]);
	return [{
		type: "unknown",
		emitName: null,
		node: null
	}];
}
/**
* Get all slots from `defineSlots` call expression.
*/
function getComponentSlotsFromDefineSlots(context, node) {
	const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
	if (typeArguments && typeArguments.params.length > 0) return require_index.getComponentSlotsFromTypeDefine(context, typeArguments.params[0]);
	return [{
		type: "unknown",
		slotName: null,
		node: null
	}];
}
/**
* Get model info from `defineModel` call expression.
*/
function getComponentModelFromDefineModel(_context, node) {
	let name = {
		modelName: "modelValue",
		node: null
	};
	let options = node.arguments[0]?.type === "SpreadElement" ? null : node.arguments[0];
	if (node.arguments.length > 0) {
		const nameNodeCandidate = skipTSAsExpression(node.arguments[0]);
		if (nameNodeCandidate.type === "Literal") {
			name = {
				modelName: String(nameNodeCandidate.value),
				node: nameNodeCandidate
			};
			options = node.arguments[1]?.type === "SpreadElement" ? null : node.arguments[1];
		}
	}
	const typeArguments = "typeArguments" in node ? node.typeArguments : node.typeParameters;
	if (typeArguments && typeArguments.params.length > 0) return {
		name,
		options,
		typeNode: typeArguments.params[0]
	};
	return {
		name,
		options,
		typeNode: null
	};
}
/**
* Get all props by looking at all component's properties
*/
function getComponentPropsFromDefine(propsNode) {
	if (propsNode.type === "ObjectExpression") return propsNode.properties.map((prop) => {
		if (!isProperty(prop)) return {
			type: "unknown",
			propName: null,
			node: prop
		};
		const propName = getStaticPropertyName(prop);
		if (propName != null) return {
			type: "object",
			key: prop.key,
			propName,
			value: skipTSAsExpression(prop.value),
			node: prop
		};
		return {
			type: "object",
			key: null,
			propName: null,
			value: skipTSAsExpression(prop.value),
			node: prop
		};
	});
	return propsNode.elements.filter(isDef).map((prop) => {
		if (prop.type === "Literal" || prop.type === "TemplateLiteral") {
			const propName = getStringLiteralValue(prop);
			if (propName != null) return {
				type: "array",
				key: prop,
				propName,
				value: null,
				node: prop
			};
		}
		return {
			type: "array",
			key: null,
			propName: null,
			value: null,
			node: prop
		};
	});
}
/**
* Get all emits by looking at all component's properties
*/
function getComponentEmitsFromDefine(emitsNode) {
	if (emitsNode.type === "ObjectExpression") return emitsNode.properties.map((prop) => {
		if (!isProperty(prop)) return {
			type: "unknown",
			key: null,
			emitName: null,
			value: null,
			node: prop
		};
		const emitName = getStaticPropertyName(prop);
		if (emitName != null) return {
			type: "object",
			key: prop.key,
			emitName,
			value: skipTSAsExpression(prop.value),
			node: prop
		};
		return {
			type: "object",
			key: null,
			emitName: null,
			value: skipTSAsExpression(prop.value),
			node: prop
		};
	});
	return emitsNode.elements.filter(isDef).map((emit) => {
		if (emit.type === "Literal" || emit.type === "TemplateLiteral") {
			const emitName = getStringLiteralValue(emit);
			if (emitName != null) return {
				type: "array",
				key: emit,
				emitName,
				value: null,
				node: emit
			};
		}
		return {
			type: "array",
			key: null,
			emitName: null,
			value: null,
			node: emit
		};
	});
}
function getObjectOrArray(context, node) {
	if (node.type === "ObjectExpression" || node.type === "ArrayExpression") return node;
	if (node.type === "Identifier") {
		const variable = (0, _eslint_community_eslint_utils.findVariable)(require_scope.getScope(context, node), node);
		if (variable != null && variable.defs.length === 1) {
			const def = variable.defs[0];
			if (def.type === "Variable" && def.parent.kind === "const" && def.node.id.type === "Identifier" && def.node.init) return getObjectOrArray(context, def.node.init);
		}
	}
	return null;
}
exports.default = utils;
exports.defineDocumentVisitor = defineDocumentVisitor;
exports.getComponentPropsFromOptions = getComponentPropsFromOptions;
exports.getCoreRule = getCoreRule;
exports.getStaticPropertyName = getStaticPropertyName;
exports.getStringLiteralValue = getStringLiteralValue;
exports.wrapCoreRule = wrapCoreRule;
