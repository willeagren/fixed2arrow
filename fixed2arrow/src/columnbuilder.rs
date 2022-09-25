use arrow::datatypes::{DataType, Field};
use std::alloc;

struct FixedField {
    len: i32,
    destinField: Field,
    srcType: DataType,
    tableId: i32
}

struct FixedRow {
    fixedField: Vec<FixedField>
}

struct FixedSizeTable<'a> {
    bytes: Vec<u8>,
    tableChunks: Vec<FixedSizeTableChunk<'a>>,
    row: &'a FixedRow,
    mem: alloc::System,
}

struct FixedSizeTableChunk<'a> {
    chunkr: i32,
    fixedSizeTable: &'a FixedSizeTable<'a>,
    columnBuilders: Vec<dyn ColumnBuilder>,
}

// ERROR: Sized is not implemented for trait ColumnBuilder... FIX THIS
pub trait ColumnBuilder {
    fn parseValue(&self, name: String) -> bool where Self: Sized;
    fn finishColumn(&self) -> bool where Self: Sized;
    fn nullify(&self) where Self: Sized;
}
