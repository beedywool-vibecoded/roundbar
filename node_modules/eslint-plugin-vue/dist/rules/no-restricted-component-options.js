const require_index = require("../utils/index.js");
const require_regexp = require("../utils/regexp.js");
// lib/rules/no-restricted-component-options.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function parseOption(option) {
	if (typeof option === "string" || Array.isArray(option)) return parseOption({ name: option });
	const steps = [];
	const names = Array.isArray(option.name) ? option.name : [option.name];
	for (const name of names) if (name === "*") steps.push({ wildcard: true });
	else {
		const matcher = require_regexp.toRegExp(name, { remove: "g" });
		steps.push({ test: (value) => matcher.test(value) });
	}
	const message = option.message;
	return {
		test: buildTester(0),
		message
	};
	function buildTester(index) {
		const step = steps[index];
		const next = index + 1;
		const hasNext = steps.length > next;
		return (node) => {
			let keyName;
			if (step.wildcard) keyName = "*";
			else {
				if (node.type !== "Property") return null;
				const name = require_index.default.getStaticPropertyName(node);
				if (!name || !step.test(name)) return null;
				keyName = name;
			}
			return {
				next: hasNext ? buildTester(next) : void 0,
				wildcard: step.wildcard,
				keyName
			};
		};
	}
}
function defaultMessage(path) {
	return `Using \`${path.join(".")}\` is not allowed.`;
}
var no_restricted_component_options_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow specific component option",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-restricted-component-options.html"
		},
		fixable: null,
		schema: {
			type: "array",
			items: { oneOf: [
				{ type: "string" },
				{
					type: "array",
					items: { type: "string" }
				},
				{
					type: "object",
					properties: {
						name: { oneOf: [{ type: "string" }, {
							type: "array",
							items: { type: "string" }
						}] },
						message: {
							type: "string",
							minLength: 1
						}
					},
					required: ["name"],
					additionalProperties: false
				}
			] },
			uniqueItems: true,
			minItems: 0
		},
		messages: { restrictedOption: "{{message}}" }
	},
	create(context) {
		if (!context.options || context.options.length === 0) return {};
		const options = context.options.map(parseOption);
		return require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, { onVueObjectEnter(node) {
			for (const option of options) verify(node, option.test, option.message);
		} }), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			if (node.arguments.length === 0) return;
			const define = node.arguments[0];
			if (define.type !== "ObjectExpression") return;
			for (const option of options) verify(define, option.test, option.message);
		} }));
		function verify(node, test, customMessage, path = []) {
			for (const prop of node.properties) {
				const result = test(prop);
				if (!result) continue;
				if (result.next) {
					if (prop.type !== "Property" || prop.value.type !== "ObjectExpression") continue;
					verify(prop.value, result.next, customMessage, [...path, result.keyName]);
				} else {
					const message = customMessage || defaultMessage([...path, result.keyName]);
					context.report({
						node: prop.type === "Property" ? prop.key : prop,
						messageId: "restrictedOption",
						data: { message }
					});
				}
			}
		}
	}
};
exports.default = no_restricted_component_options_default;
