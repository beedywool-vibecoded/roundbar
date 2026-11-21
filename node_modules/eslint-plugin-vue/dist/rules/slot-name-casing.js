const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
// lib/rules/slot-name-casing.ts
/**
* Checks whether the given string is a single word.
*/
function isSingleWord(str) {
	return /^[a-z]+$/u.test(str);
}
const allowedCaseOptions = [
	"camelCase",
	"kebab-case",
	"singleword"
];
var slot_name_casing_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce specific casing for slot names",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/slot-name-casing.html"
		},
		fixable: null,
		schema: [{ enum: allowedCaseOptions }],
		messages: { invalidCase: "Slot name \"{{name}}\" is not {{caseType}}." }
	},
	create(context) {
		const option = context.options[0];
		const caseType = allowedCaseOptions.includes(option) ? option : "camelCase";
		const checker = caseType === "singleword" ? isSingleWord : require_casing.getChecker(caseType);
		function processSlotNode(node) {
			const name = node.value?.value;
			if (name && !checker(name)) context.report({
				node,
				loc: node.loc,
				messageId: "invalidCase",
				data: {
					name,
					caseType
				}
			});
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='slot']"(node) {
			const slotName = require_index.default.getAttribute(node, "name");
			if (slotName) processSlotNode(slotName);
		} });
	}
};
exports.default = slot_name_casing_default;
