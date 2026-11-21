const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-props-default-this.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function isFunctionIdentifier(node) {
	return node !== null && node.type === "Identifier" && node.name === "Function";
}
function hasFunctionType(node) {
	return isFunctionIdentifier(node) || node.type === "ArrayExpression" && node.elements.some(isFunctionIdentifier);
}
var no_deprecated_props_default_this_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow deprecated `this` access in props default function (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-props-default-this.html"
		},
		fixable: null,
		schema: [],
		messages: { deprecated: "Props default value factory functions no longer have access to `this`." }
	},
	create(context) {
		const propsDefault = /* @__PURE__ */ new Set();
		let scopeStack = null;
		function onFunctionEnter(node) {
			if (node.type === "ArrowFunctionExpression") return;
			if (scopeStack) scopeStack = {
				upper: scopeStack,
				node,
				propDefault: false
			};
			else if (node.type === "FunctionExpression" && propsDefault.has(node)) scopeStack = {
				upper: scopeStack,
				node,
				propDefault: true
			};
		}
		function onFunctionExit(node) {
			if (scopeStack && scopeStack.node === node) scopeStack = scopeStack.upper;
		}
		return require_index.default.defineVueVisitor(context, {
			onVueObjectEnter(node) {
				for (const prop of require_index.default.getComponentPropsFromOptions(node)) {
					if (prop.type !== "object" || prop.value.type !== "ObjectExpression") continue;
					const def = require_index.default.findProperty(prop.value, "default");
					if (!def) continue;
					const type = require_index.default.findProperty(prop.value, "type");
					if (type && hasFunctionType(type.value) || def.value.type !== "FunctionExpression") continue;
					propsDefault.add(def.value);
				}
			},
			":function": onFunctionEnter,
			":function:exit": onFunctionExit,
			ThisExpression(node) {
				if (scopeStack && scopeStack.propDefault) context.report({
					node,
					messageId: "deprecated"
				});
			}
		});
	}
};
exports.default = no_deprecated_props_default_this_default;
