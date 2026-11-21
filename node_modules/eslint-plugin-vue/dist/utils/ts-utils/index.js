const require_ts_ast = require("./ts-ast.js");
const require_ts_types = require("./ts-types.js");
// lib/utils/ts-utils/index.ts
/**
* Get all props by looking at all component's properties
*/
function getComponentPropsFromTypeDefine(context, propsNode) {
	const result = [];
	for (const defNode of require_ts_ast.flattenTypeNodes(context, propsNode)) if (require_ts_ast.isTSInterfaceBody(defNode) || require_ts_ast.isTSTypeLiteral(defNode)) result.push(...require_ts_ast.extractRuntimeProps(context, defNode));
	else result.push(...require_ts_types.getComponentPropsFromTypeDefineTypes(context, defNode));
	return result;
}
/**
* Get all emits by looking at all component's properties
*/
function getComponentEmitsFromTypeDefine(context, emitsNode) {
	const result = [];
	for (const defNode of require_ts_ast.flattenTypeNodes(context, emitsNode)) if (require_ts_ast.isTSInterfaceBody(defNode) || require_ts_ast.isTSTypeLiteralOrTSFunctionType(defNode)) result.push(...require_ts_ast.extractRuntimeEmits(defNode));
	else result.push(...require_ts_types.getComponentEmitsFromTypeDefineTypes(context, defNode));
	return result;
}
/**
* Get all slots by looking at all component's properties
*/
function getComponentSlotsFromTypeDefine(context, slotsNode) {
	const result = [];
	for (const defNode of require_ts_ast.flattenTypeNodes(context, slotsNode)) if (require_ts_ast.isTSInterfaceBody(defNode) || require_ts_ast.isTSTypeLiteral(defNode)) result.push(...require_ts_ast.extractRuntimeSlots(defNode));
	else result.push(...require_ts_types.getComponentSlotsFromTypeDefineTypes(context, defNode));
	return result;
}
exports.getComponentEmitsFromTypeDefine = getComponentEmitsFromTypeDefine;
exports.getComponentPropsFromTypeDefine = getComponentPropsFromTypeDefine;
exports.getComponentSlotsFromTypeDefine = getComponentSlotsFromTypeDefine;
exports.isExhaustiveSwitch = require_ts_types.isExhaustiveSwitch;
exports.isRefObjectType = require_ts_types.isRefObjectType;
exports.isTypeNode = require_ts_ast.isTypeNode;
