const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
const require_regexp = require("../utils/regexp.js");
// lib/rules/component-name-in-template-casing.ts
/**
* @author Yosuke Ota
* issue https://github.com/vuejs/eslint-plugin-vue/issues/250
*/
const allowedCaseOptions = ["PascalCase", "kebab-case"];
const defaultCase = "PascalCase";
/**
* Checks whether the given variable is the type-only import object.
*/
function isTypeOnlyImport(variable) {
	return variable.defs.length > 0 && variable.defs.every((def) => def.type === "ImportBinding" && (def.parent.importKind === "type" || def.node.type === "ImportSpecifier" && def.node.importKind === "type"));
}
/**
* Gets the component name of the given tag name, i.e. the part before the
* first dot for a namespaced component (e.g. `Form` of `Form.Input`).
*/
function getComponentName(rawName) {
	return rawName.split(".", 1)[0];
}
var component_name_in_template_casing_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce specific casing for the component naming style in template",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/component-name-in-template-casing.html"
		},
		fixable: "code",
		schema: [{ enum: allowedCaseOptions }, {
			type: "object",
			properties: {
				globals: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				},
				ignores: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				},
				registeredComponentsOnly: { type: "boolean" }
			},
			additionalProperties: false
		}],
		messages: { incorrectCase: "Component name \"{{name}}\" is not {{caseType}}." }
	},
	create(context) {
		const caseOption = context.options[0];
		const options = context.options[1] || {};
		const caseType = allowedCaseOptions.includes(caseOption) ? caseOption : defaultCase;
		const isIgnored = require_regexp.toRegExpGroupMatcher(options.ignores);
		const globalStrings = [];
		const globalPatterns = [];
		const globals = options.globals || [];
		for (const global of globals) if (require_regexp.isRegExp(global)) globalPatterns.push(global);
		else globalStrings.push(global);
		const isGlobalPattern = require_regexp.toRegExpGroupMatcher(globalPatterns);
		const shouldCheckRegisteredComponentsOnly = options.registeredComponentsOnly !== false;
		const sourceCode = context.sourceCode;
		const tokens = sourceCode.parserServices.getTemplateBodyTokenStore && sourceCode.parserServices.getTemplateBodyTokenStore();
		const registeredComponents = new Set(globalStrings.map(require_casing.pascalCase));
		if (require_index.default.isScriptSetup(context)) {
			const globalScope = context.sourceCode.scopeManager.globalScope;
			if (globalScope) {
				const moduleScope = globalScope.childScopes.find((scope) => scope.type === "module");
				const moduleVariables = moduleScope && moduleScope.variables || [];
				for (const variable of moduleVariables) if (!isTypeOnlyImport(variable)) registeredComponents.add(variable.name);
			}
		}
		/**
		* Checks whether the given node is the verification target node.
		*/
		function isVerifyTarget(node) {
			if (isIgnored(node.rawName) || !require_index.default.isHtmlElementNode(node) && !require_index.default.isSvgElementNode(node) && !require_index.default.isMathElementNode(node) || require_index.default.isHtmlWellKnownElementName(node.rawName) || require_index.default.isSvgWellKnownElementName(node.rawName) || require_index.default.isMathWellKnownElementName(node.rawName) || require_index.default.isVueBuiltInElementName(node.rawName)) return false;
			return shouldCheckRegisteredComponentsOnly ? registeredComponents.has(require_casing.pascalCase(getComponentName(node.rawName))) || isGlobalPattern(node.rawName) : true;
		}
		let hasInvalidEOF = false;
		return require_index.default.defineTemplateBodyVisitor(context, { VElement(node) {
			if (hasInvalidEOF || !isVerifyTarget(node)) return;
			const name = node.rawName;
			const componentName = getComponentName(name);
			if (require_casing.getChecker(caseType)(componentName)) return;
			const startTag = node.startTag;
			const open = tokens.getFirstToken(startTag);
			const casingName = require_casing.getExactConverter(caseType)(componentName) + name.slice(componentName.length);
			context.report({
				node: open,
				loc: open.loc,
				messageId: "incorrectCase",
				data: {
					name,
					caseType
				},
				*fix(fixer) {
					yield fixer.replaceText(open, `<${casingName}`);
					const endTag = node.endTag;
					if (!endTag) return;
					const endTagOpen = tokens.getFirstToken(endTag);
					yield fixer.replaceText(endTagOpen, `</${casingName}`);
				}
			});
		} }, {
			Program(node) {
				hasInvalidEOF = require_index.default.hasInvalidEOF(node);
			},
			...shouldCheckRegisteredComponentsOnly ? require_index.default.executeOnVue(context, (obj) => {
				for (const n of require_index.default.getRegisteredComponents(obj)) registeredComponents.add(n.name);
			}) : {}
		});
	}
};
exports.default = component_name_in_template_casing_default;
