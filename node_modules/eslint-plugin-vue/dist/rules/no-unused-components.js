const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
// lib/rules/no-unused-components.ts
/**
* @fileoverview Report used components
* @author Michał Sajnóg
*/
var no_unused_components_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow registering components that are not used inside templates",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-unused-components.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { ignoreWhenBindingPresent: { type: "boolean" } },
			additionalProperties: false
		}],
		messages: { unused: "The \"{{name}}\" component has been registered but not used." }
	},
	create(context) {
		const ignoreWhenBindingPresent = (context.options[0] || {}).ignoreWhenBindingPresent ?? true;
		const usedComponents = /* @__PURE__ */ new Set();
		let registeredComponents = [];
		let shouldIgnoreReporting = false;
		let templateLocation;
		return require_index.default.defineTemplateBodyVisitor(context, {
			VElement(node) {
				if (!require_index.default.isHtmlElementNode(node) && !require_index.default.isSvgElementNode(node) && !require_index.default.isMathElementNode(node) || require_index.default.isHtmlWellKnownElementName(node.rawName) || require_index.default.isSvgWellKnownElementName(node.rawName) || require_index.default.isMathWellKnownElementName(node.rawName)) return;
				usedComponents.add(node.rawName);
			},
			"VAttribute[directive=true][key.name.name='bind'][key.argument.name='is'], VAttribute[directive=true][key.name.name='is']"(node) {
				if (!node.value || node.value.type !== "VExpressionContainer" || !node.value.expression) return;
				if (node.value.expression.type === "Literal") usedComponents.add(node.value.expression.value);
				else if (ignoreWhenBindingPresent) shouldIgnoreReporting = true;
			},
			"VAttribute[directive=false][key.name='is']"(node) {
				if (!node.value) return;
				const value = node.value.value.startsWith("vue:") ? node.value.value.slice(4) : node.value.value;
				usedComponents.add(value);
			},
			"VElement[name='template']"(node) {
				templateLocation ||= node.loc.start;
			},
			"VElement[name='template']:exit"(node) {
				if (shouldIgnoreReporting || node.loc.start !== templateLocation || require_index.default.hasAttribute(node, "src")) return;
				for (const { node, name } of registeredComponents) {
					if (require_casing.isPascalCase(name) || require_casing.isCamelCase(name)) {
						if ([...usedComponents].some((n) => !n.includes("_") && (name === require_casing.pascalCase(n) || name === require_casing.camelCase(n)))) continue;
					} else if (usedComponents.has(name)) continue;
					context.report({
						node,
						messageId: "unused",
						data: { name }
					});
				}
			}
		}, require_index.default.executeOnVue(context, (obj) => {
			registeredComponents = require_index.default.getRegisteredComponents(obj);
		}));
	}
};
exports.default = no_unused_components_default;
