const require_index = require("../utils/index.js");
// lib/rules/require-explicit-slots.ts
function getSlotsName(node) {
	if (node.type !== "TSMethodSignature" && node.type !== "TSPropertySignature") return null;
	const key = node.key;
	if (key.type === "Literal") return typeof key.value === "string" ? key.value : null;
	return key.type === "Identifier" ? key.name : null;
}
function getSlotNameNode(node) {
	return node.startTag.attributes.find((node) => !node.directive && node.key.name === "name" || node.directive && node.key.name.name === "bind" && node.key.argument?.type === "VIdentifier" && node.key.argument?.name === "name");
}
var require_explicit_slots_default = {
	meta: {
		type: "problem",
		docs: {
			description: "require slots to be explicitly defined",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/require-explicit-slots.html"
		},
		fixable: null,
		schema: [],
		messages: {
			requireExplicitSlots: "Slots must be explicitly defined.",
			alreadyDefinedSlot: "Slot {{slotName}} is already defined."
		}
	},
	create(context) {
		const sourceCode = context.sourceCode;
		const documentFragment = sourceCode.parserServices.getDocumentFragment && sourceCode.parserServices.getDocumentFragment();
		if (!documentFragment) return {};
		if (documentFragment.children.filter((element) => require_index.default.isVElement(element) && element.name === "script").every((script) => !require_index.default.hasAttribute(script, "lang", "ts"))) return {};
		const slotsDefined = /* @__PURE__ */ new Set();
		function reportMissingSlot(node, slotName) {
			if (!slotsDefined.has(slotName)) context.report({
				node,
				messageId: "requireExplicitSlots"
			});
		}
		return require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, { onDefineSlotsEnter(_node, slots) {
			for (const slot of slots) {
				if (!slot.slotName) continue;
				if (slotsDefined.has(slot.slotName)) context.report({
					node: slot.node,
					messageId: "alreadyDefinedSlot",
					data: { slotName: slot.slotName }
				});
				else slotsDefined.add(slot.slotName);
			}
		} }), require_index.default.executeOnVue(context, (obj) => {
			const slotsProperty = require_index.default.findProperty(obj, "slots");
			if (!slotsProperty) return;
			const slotsTypeHelper = slotsProperty.value.type === "TSAsExpression" && slotsProperty.value.typeAnnotation?.type === "TSTypeReference" && slotsProperty.value.typeAnnotation.typeName.type === "Identifier" && slotsProperty.value.typeAnnotation.typeName.name === "SlotsType" && slotsProperty.value.typeAnnotation;
			if (!slotsTypeHelper) return;
			const param = ("typeArguments" in slotsTypeHelper ? slotsTypeHelper.typeArguments : slotsTypeHelper.typeParameters)?.params[0];
			if (!param) return;
			if (param.type === "TSTypeLiteral") for (const memberNode of param.members) {
				const slotName = getSlotsName(memberNode);
				if (!slotName) continue;
				if (slotsDefined.has(slotName)) context.report({
					node: memberNode,
					messageId: "alreadyDefinedSlot",
					data: { slotName }
				});
				else slotsDefined.add(slotName);
			}
		}), require_index.default.defineTemplateBodyVisitor(context, { "VElement[name='slot']"(node) {
			const nameNode = getSlotNameNode(node);
			if (!nameNode) {
				reportMissingSlot(node, "default");
				return;
			}
			if (nameNode.directive) {
				const expression = nameNode.value?.expression;
				if (!expression || !require_index.default.isStringLiteral(expression)) return;
				reportMissingSlot(node, require_index.default.getStringLiteralValue(expression) || void 0);
			} else reportMissingSlot(node, nameNode.value?.value);
		} }));
	}
};
exports.default = require_explicit_slots_default;
