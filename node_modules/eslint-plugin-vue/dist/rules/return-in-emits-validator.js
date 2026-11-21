const require_index = require("../utils/index.js");
// lib/rules/return-in-emits-validator.ts
/**
* Checks if the given node value is falsy.
*/
function isFalsy(node) {
	if (node.type === "Literal") {
		if (node.bigint) return node.bigint === "0";
		if (!node.value) return true;
	}
	return node.type === "Identifier" && (node.name === "undefined" || node.name === "NaN");
}
var return_in_emits_validator_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce that a return statement is present in emits validator",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/return-in-emits-validator.html"
		},
		fixable: null,
		schema: [],
		messages: {
			expectedTrue: "Expected to return a true value in \"{{name}}\" emits validator.",
			expectedBoolean: "Expected to return a boolean value in \"{{name}}\" emits validator."
		}
	},
	create(context) {
		const emitsValidators = [];
		let scopeStack = null;
		function processEmits(emits) {
			for (const emit of emits) {
				if (emit.type !== "object" || !emit.value || emit.value.type !== "FunctionExpression" && emit.value.type !== "ArrowFunctionExpression") continue;
				emitsValidators.push(emit);
			}
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefineEmitsEnter(_node, emits) {
			processEmits(emits);
		} }), require_index.default.defineVueVisitor(context, { onVueObjectEnter(obj) {
			processEmits(require_index.default.getComponentEmitsFromOptions(obj));
		} }), {
			":function"(node) {
				scopeStack = {
					upper: scopeStack,
					functionNode: node,
					hasReturnValue: false,
					possibleOfReturnTrue: false
				};
				if (!(node.type === "ArrowFunctionExpression" && node.expression)) return;
				scopeStack.hasReturnValue = true;
				if (!isFalsy(node.body)) scopeStack.possibleOfReturnTrue = true;
			},
			ReturnStatement(node) {
				if (!scopeStack || !node.argument) return;
				scopeStack.hasReturnValue = true;
				if (!isFalsy(node.argument)) scopeStack.possibleOfReturnTrue = true;
			},
			":function:exit"(node) {
				if (scopeStack && !scopeStack.possibleOfReturnTrue) {
					const emits = emitsValidators.find((e) => e.value === node);
					if (emits) context.report({
						node,
						messageId: scopeStack.hasReturnValue ? "expectedTrue" : "expectedBoolean",
						data: { name: emits.emitName || "Unknown" }
					});
				}
				scopeStack &&= scopeStack.upper;
			}
		});
	}
};
exports.default = return_in_emits_validator_default;
