namespace go knowledge

struct CommonRes {
    1: bool success
}

struct CreateKnowledgeBaseReq {
    1: i64 owner_id
    2: string name
    3: optional string description
}

struct CreateKnowledgeBaseRes {
    1: bool success
    2: i64 kb_id
}

struct KnowledgeBaseInfo {
    1: i64 kb_id
    2: i64 owner_id
    3: string name
    4: string description
    5: i32 doc_count
    6: i32 chunk_count
    7: i64 created_at
}

struct GetKnowledgeBaseReq {
    1: i64 kb_id
}

struct GetKnowledgeBaseRes {
    1: bool success
    2: optional KnowledgeBaseInfo kb_info
}

struct GetUserKnowledgeBasesReq {
    1: i64 owner_id
}

struct GetUserKnowledgeBasesRes {
    1: bool success
    2: list<KnowledgeBaseInfo> knowledge_bases
}

struct UpdateKnowledgeBaseReq {
    1: i64 kb_id
    2: i64 operator_id
    3: optional string name
    4: optional string description
}

struct DeleteKnowledgeBaseReq {
    1: i64 kb_id
    2: i64 operator_id
}

struct AddDocumentReq {
    1: i64 kb_id
    2: i64 operator_id
    3: string file_name
    4: string file_url
    5: string file_type
    6: i64 file_size
}

struct AddDocumentRes {
    1: bool success
    2: i64 doc_id
}

struct DocumentInfo {
    1: i64 doc_id
    2: i64 kb_id
    3: string file_name
    4: string file_url
    5: string file_type
    6: i64 file_size
    7: string status
    8: i32 chunk_count
    9: optional string error_message
    10: i64 created_at
}

struct GetDocumentsReq {
    1: i64 kb_id
}

struct GetDocumentsRes {
    1: bool success
    2: list<DocumentInfo> documents
}

struct DeleteDocumentReq {
    1: i64 doc_id
    2: i64 operator_id
}

struct RetryDocumentReq {
    1: i64 doc_id
    2: i64 operator_id
}

struct BindKnowledgeBaseReq {
    1: i64 bot_id
    2: i64 operator_id
    3: i64 kb_id
}

struct UnbindKnowledgeBaseReq {
    1: i64 bot_id
    2: i64 operator_id
    3: i64 kb_id
}

struct GetBotKnowledgeBasesReq {
    1: i64 bot_id
}

struct GetBotKnowledgeBasesRes {
    1: bool success
    2: list<KnowledgeBaseInfo> knowledge_bases
}

struct SearchKnowledgeReq {
    1: list<i64> kb_ids
    2: string query
    3: i32 top_k
}

struct KnowledgeChunk {
    1: i64 chunk_id
    2: i64 kb_id
    3: i64 doc_id
    4: string content
    5: i32 chunk_index
    6: string source
    7: optional i32 page_number
    8: double score
}

struct SearchKnowledgeRes {
    1: bool success
    2: list<KnowledgeChunk> chunks
}

struct BindSystemKnowledgeBaseReq {
    1: i64 bot_id
    2: i64 kb_id
}

struct AddSystemDocumentReq {
    1: i64 kb_id
    2: string file_name
    3: string file_url
    4: string file_type
    5: i64 file_size
}

service KnowledgeService {
    CreateKnowledgeBaseRes CreateKnowledgeBase(1: CreateKnowledgeBaseReq req)
    GetKnowledgeBaseRes GetKnowledgeBase(1: GetKnowledgeBaseReq req)
    GetUserKnowledgeBasesRes GetUserKnowledgeBases(1: GetUserKnowledgeBasesReq req)
    CommonRes UpdateKnowledgeBase(1: UpdateKnowledgeBaseReq req)
    CommonRes DeleteKnowledgeBase(1: DeleteKnowledgeBaseReq req)

    AddDocumentRes AddDocument(1: AddDocumentReq req)
    GetDocumentsRes GetDocuments(1: GetDocumentsReq req)
    CommonRes DeleteDocument(1: DeleteDocumentReq req)
    CommonRes RetryDocument(1: RetryDocumentReq req)

    CommonRes BindKnowledgeBase(1: BindKnowledgeBaseReq req)
    CommonRes UnbindKnowledgeBase(1: UnbindKnowledgeBaseReq req)
    GetBotKnowledgeBasesRes GetBotKnowledgeBases(1: GetBotKnowledgeBasesReq req)

    SearchKnowledgeRes SearchKnowledge(1: SearchKnowledgeReq req)

    CommonRes BindSystemKnowledgeBase(1: BindSystemKnowledgeBaseReq req)
    AddDocumentRes AddSystemDocument(1: AddSystemDocumentReq req)
}
